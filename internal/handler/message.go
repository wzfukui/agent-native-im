package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wzfukui/agent-native-im/internal/auth"
	"github.com/wzfukui/agent-native-im/internal/model"
)

type sendMessageRequest struct {
	ConversationID int64               `json:"conversation_id" binding:"required"`
	ContentType    string              `json:"content_type,omitempty"`
	Layers         model.MessageLayers `json:"layers"`
	Attachments    []model.Attachment  `json:"attachments,omitempty"`
	StreamID       string              `json:"stream_id,omitempty"`
	Mentions       []int64             `json:"mentions,omitempty"`
	ReplyTo        *int64              `json:"reply_to,omitempty"`
}

func (s *Server) HandleSendMessage(c *gin.Context) {
	var req sendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	// Verify participant and check observer role
	ok, err := s.Store.IsParticipant(ctx, req.ConversationID, entityID)
	if err != nil || !ok {
		Fail(c, http.StatusForbidden, "not a participant of this conversation")
		return
	}
	participant, err := s.Store.GetParticipant(ctx, req.ConversationID, entityID)
	if err == nil && participant != nil && participant.Role == model.RoleObserver {
		Fail(c, http.StatusForbidden, "observers cannot send messages")
		return
	}

	// Validate mentions are actual participants
	for _, mentionID := range req.Mentions {
		isMember, err := s.Store.IsParticipant(ctx, req.ConversationID, mentionID)
		if err != nil || !isMember {
			Fail(c, http.StatusBadRequest, fmt.Sprintf("mentioned entity %d is not a participant", mentionID))
			return
		}
	}

	contentType := model.ContentType(req.ContentType)
	if contentType == "" {
		contentType = model.ContentText
	}

	msg := &model.Message{
		ConversationID: req.ConversationID,
		SenderID:       entityID,
		StreamID:       req.StreamID,
		ContentType:    contentType,
		Layers:         req.Layers,
		Attachments:    req.Attachments,
		Mentions:       req.Mentions,
		ReplyTo:        req.ReplyTo,
	}

	if err := s.Store.CreateMessage(ctx, msg); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to save message")
		return
	}

	_ = s.Store.TouchConversation(ctx, req.ConversationID)

	// Populate sender info
	sender, err := s.Store.GetEntityByID(ctx, entityID)
	if err == nil {
		msg.SenderType = string(sender.EntityType)
		msg.Sender = sender
	}

	// Broadcast via WebSocket
	if s.Hub != nil {
		s.Hub.BroadcastMessage(msg)
	}

	// Deliver webhooks
	if s.Webhook != nil {
		s.Webhook.DeliverAsync(msg)
	}

	OK(c, http.StatusCreated, msg)
}

const revokeWindow = 2 * time.Minute

// HandleRevokeMessage revokes (soft-deletes) a message within the allowed time window.
func (s *Server) HandleRevokeMessage(c *gin.Context) {
	msgID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid message id")
		return
	}

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	msg, err := s.Store.GetMessageByID(ctx, msgID)
	if err != nil {
		Fail(c, http.StatusNotFound, "message not found")
		return
	}

	if msg.SenderID != entityID {
		Fail(c, http.StatusForbidden, "can only revoke your own messages")
		return
	}

	if msg.RevokedAt != nil {
		Fail(c, http.StatusBadRequest, "message already revoked")
		return
	}

	if time.Since(msg.CreatedAt) > revokeWindow {
		Fail(c, http.StatusForbidden, "revoke window expired (2 minutes)")
		return
	}

	if err := s.Store.RevokeMessage(ctx, msgID); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to revoke message")
		return
	}

	// Broadcast revocation event
	if s.Hub != nil {
		s.Hub.BroadcastEvent(msg.ConversationID, "message.revoked", gin.H{
			"message_id":      msgID,
			"conversation_id": msg.ConversationID,
		})
	}

	OK(c, http.StatusOK, "message revoked")
}

// HandleSearchMessages searches messages in a conversation using full-text search.
func (s *Server) HandleSearchMessages(c *gin.Context) {
	convID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid conversation id")
		return
	}

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	ok, err := s.Store.IsParticipant(ctx, convID, entityID)
	if err != nil || !ok {
		Fail(c, http.StatusForbidden, "not a participant of this conversation")
		return
	}

	query := c.Query("q")
	if query == "" {
		Fail(c, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit <= 0 {
		limit = 20
	} else if limit > 100 {
		limit = 100
	}

	msgs, err := s.Store.SearchMessages(ctx, convID, query, limit)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "search failed")
		return
	}
	if msgs == nil {
		msgs = []*model.Message{}
	}

	OK(c, http.StatusOK, gin.H{
		"messages": msgs,
		"query":    query,
	})
}

// HandleInteractionResponse records a user's response to an interaction message.
func (s *Server) HandleInteractionResponse(c *gin.Context) {
	msgID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid message id")
		return
	}

	var req struct {
		Value string `json:"value" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "value is required")
		return
	}

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	msg, err := s.Store.GetMessageByID(ctx, msgID)
	if err != nil {
		Fail(c, http.StatusNotFound, "message not found")
		return
	}

	// Verify the message has an interaction layer
	if msg.Layers.Interaction == nil {
		Fail(c, http.StatusBadRequest, "message has no interaction")
		return
	}

	// Verify responder is a participant
	ok, err := s.Store.IsParticipant(ctx, msg.ConversationID, entityID)
	if err != nil || !ok {
		Fail(c, http.StatusForbidden, "not a participant of this conversation")
		return
	}

	// Broadcast interaction response event
	if s.Hub != nil {
		responder, _ := s.Store.GetEntityByID(ctx, entityID)
		responderName := "someone"
		if responder != nil {
			responderName = responder.DisplayName
			if responderName == "" {
				responderName = responder.Name
			}
		}
		s.Hub.BroadcastEvent(msg.ConversationID, "message.interaction_response", map[string]interface{}{
			"message_id":      msgID,
			"conversation_id": msg.ConversationID,
			"entity_id":       entityID,
			"entity_name":     responderName,
			"value":           req.Value,
			"responded_at":    time.Now(),
		})
	}

	OK(c, http.StatusOK, gin.H{"message_id": msgID, "value": req.Value})
}

// HandleEditMessage allows the sender to edit their own message within 5 minutes.
func (s *Server) HandleEditMessage(c *gin.Context) {
	msgID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid message id")
		return
	}

	var req struct {
		Layers model.MessageLayers `json:"layers"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	msg, err := s.Store.GetMessageByID(ctx, msgID)
	if err != nil {
		Fail(c, http.StatusNotFound, "message not found")
		return
	}

	if msg.SenderID != entityID {
		Fail(c, http.StatusForbidden, "can only edit your own messages")
		return
	}

	if msg.RevokedAt != nil {
		Fail(c, http.StatusBadRequest, "cannot edit revoked message")
		return
	}

	if time.Since(msg.CreatedAt) > 5*time.Minute {
		Fail(c, http.StatusForbidden, "edit window expired (5 minutes)")
		return
	}

	// Update the message layers
	msg.Layers = req.Layers

	if err := s.Store.UpdateMessage(ctx, msg); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to edit message")
		return
	}

	// Broadcast edit event
	if s.Hub != nil {
		sender, _ := s.Store.GetEntityByID(ctx, entityID)
		if sender != nil {
			msg.SenderType = string(sender.EntityType)
			msg.Sender = sender
		}
		s.Hub.BroadcastEvent(msg.ConversationID, "message.updated", map[string]interface{}{
			"message": msg,
		})
	}

	OK(c, http.StatusOK, msg)
}

func (s *Server) HandleListMessages(c *gin.Context) {
	convID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid conversation id")
		return
	}

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	// Verify participant
	ok, err := s.Store.IsParticipant(ctx, convID, entityID)
	if err != nil || !ok {
		Fail(c, http.StatusForbidden, "not a participant of this conversation")
		return
	}

	before, _ := strconv.ParseInt(c.DefaultQuery("before", "0"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit <= 0 {
		limit = 20
	} else if limit > 100 {
		limit = 100
	}

	msgs, err := s.Store.ListMessages(ctx, convID, before, limit)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to list messages")
		return
	}

	// Populate sender info for each message
	entityCache := make(map[int64]*model.Entity)
	for _, msg := range msgs {
		sender, ok := entityCache[msg.SenderID]
		if !ok {
			sender, err = s.Store.GetEntityByID(ctx, msg.SenderID)
			if err == nil {
				entityCache[msg.SenderID] = sender
			}
		}
		if sender != nil {
			msg.SenderType = string(sender.EntityType)
			msg.Sender = sender
		}
	}

	if msgs == nil {
		msgs = []*model.Message{}
	}
	hasMore := len(msgs) == limit

	OK(c, http.StatusOK, gin.H{
		"messages": msgs,
		"has_more": hasMore,
	})
}
