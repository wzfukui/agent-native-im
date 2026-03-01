package handler

import (
	"net/http"
	"strconv"

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
