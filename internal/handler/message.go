package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/wzfukui/agent-native-im/internal/model"
)

type sendMessageRequest struct {
	ConversationID int64               `json:"conversation_id" binding:"required"`
	Layers         model.MessageLayers `json:"layers"`
	StreamID       string              `json:"stream_id,omitempty"`
}

func (s *Server) HandleSendMessage(c *gin.Context) {
	var req sendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	senderType := c.GetString("senderType")
	senderID := c.GetInt64("senderID")

	// Verify conversation access
	conv, err := s.Store.GetConversation(c.Request.Context(), req.ConversationID)
	if err != nil {
		Fail(c, http.StatusNotFound, "conversation not found")
		return
	}

	if senderType == "user" && conv.UserID != senderID {
		Fail(c, http.StatusForbidden, "access denied")
		return
	}
	if senderType == "bot" && conv.BotID != senderID {
		Fail(c, http.StatusForbidden, "access denied")
		return
	}

	msg := &model.Message{
		ConversationID: req.ConversationID,
		StreamID:       req.StreamID,
		SenderType:     senderType,
		SenderID:       senderID,
		Layers:         req.Layers,
	}

	if err := s.Store.CreateMessage(c.Request.Context(), msg); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to save message")
		return
	}

	// Update conversation timestamp
	_ = s.Store.TouchConversation(c.Request.Context(), req.ConversationID)

	// Broadcast via WebSocket hub
	if s.Hub != nil {
		s.Hub.BroadcastMessage(msg)
	}

	OK(c, http.StatusCreated, msg)
}

func (s *Server) HandleListMessages(c *gin.Context) {
	convID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid conversation id")
		return
	}

	// Verify access
	conv, err := s.Store.GetConversation(c.Request.Context(), convID)
	if err != nil {
		Fail(c, http.StatusNotFound, "conversation not found")
		return
	}

	senderType := c.GetString("senderType")
	senderID := c.GetInt64("senderID")
	if senderType == "user" && conv.UserID != senderID {
		Fail(c, http.StatusForbidden, "access denied")
		return
	}
	if senderType == "bot" && conv.BotID != senderID {
		Fail(c, http.StatusForbidden, "access denied")
		return
	}

	before, _ := strconv.ParseInt(c.DefaultQuery("before", "0"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	msgs, err := s.Store.ListMessages(c.Request.Context(), convID, before, limit)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to list messages")
		return
	}

	hasMore := len(msgs) == limit

	OK(c, http.StatusOK, gin.H{
		"messages": msgs,
		"has_more": hasMore,
	})
}
