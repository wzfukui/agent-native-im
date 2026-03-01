package handler

import (
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

	// Verify participant
	ok, err := s.Store.IsParticipant(ctx, req.ConversationID, entityID)
	if err != nil || !ok {
		Fail(c, http.StatusForbidden, "not a participant of this conversation")
		return
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

	OK(c, http.StatusOK, gin.H{
		"messages": msgs,
		"query":    query,
	})
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

	hasMore := len(msgs) == limit

	OK(c, http.StatusOK, gin.H{
		"messages": msgs,
		"has_more": hasMore,
	})
}
