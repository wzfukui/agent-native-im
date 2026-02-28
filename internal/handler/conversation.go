package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/wzfukui/agent-native-im/internal/model"
)

type createConversationRequest struct {
	BotID int64  `json:"bot_id" binding:"required"`
	Title string `json:"title"`
}

func (s *Server) HandleCreateConversation(c *gin.Context) {
	senderType := c.GetString("senderType")
	if senderType != "user" {
		Fail(c, http.StatusForbidden, "only users can create conversations")
		return
	}

	var req createConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "bot_id is required")
		return
	}

	userID := c.GetInt64("senderID")

	// Verify bot exists
	bot, err := s.Store.GetBotByID(c.Request.Context(), req.BotID)
	if err != nil {
		Fail(c, http.StatusBadRequest, "bot not found")
		return
	}

	title := req.Title
	if title == "" {
		title = "New conversation with " + bot.Name
	}

	conv := &model.Conversation{
		UserID: userID,
		BotID:  req.BotID,
		Title:  title,
	}

	if err := s.Store.CreateConversation(c.Request.Context(), conv); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to create conversation")
		return
	}

	// Notify WebSocket hub about new conversation
	if s.Hub != nil {
		s.Hub.NotifyNewConversation(conv.ID, userID, req.BotID)
	}

	OK(c, http.StatusCreated, conv)
}

func (s *Server) HandleListConversations(c *gin.Context) {
	senderType := c.GetString("senderType")
	senderID := c.GetInt64("senderID")

	var convs []model.Conversation
	var err error

	if senderType == "user" {
		convs, err = s.Store.ListConversationsByUser(c.Request.Context(), senderID)
	} else {
		convs, err = s.Store.ListConversationsByBot(c.Request.Context(), senderID)
	}

	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to list conversations")
		return
	}

	OK(c, http.StatusOK, convs)
}

func (s *Server) HandleGetConversation(c *gin.Context) {
	convID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid conversation id")
		return
	}

	conv, err := s.Store.GetConversation(c.Request.Context(), convID)
	if err != nil {
		Fail(c, http.StatusNotFound, "conversation not found")
		return
	}

	// Verify access
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

	OK(c, http.StatusOK, conv)
}
