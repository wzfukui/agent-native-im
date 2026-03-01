package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wzfukui/agent-native-im/internal/auth"
	"github.com/wzfukui/agent-native-im/internal/model"
)

type createWebhookRequest struct {
	URL    string   `json:"url" binding:"required"`
	Events []string `json:"events"`
}

func (s *Server) HandleCreateWebhook(c *gin.Context) {
	var req createWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "url is required")
		return
	}

	events := req.Events
	if len(events) == 0 {
		events = []string{"message.new"}
	}

	wh := &model.Webhook{
		EntityID: auth.GetEntityID(c),
		URL:      req.URL,
		Secret:   uuid.New().String(),
		Events:   events,
		Status:   "active",
	}

	if err := s.Store.CreateWebhook(c.Request.Context(), wh); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to create webhook")
		return
	}

	OK(c, http.StatusCreated, gin.H{
		"webhook": wh,
		"secret":  wh.Secret, // shown only once
	})
}

func (s *Server) HandleListWebhooks(c *gin.Context) {
	webhooks, err := s.Store.ListWebhooksByEntity(c.Request.Context(), auth.GetEntityID(c))
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to list webhooks")
		return
	}

	OK(c, http.StatusOK, webhooks)
}

func (s *Server) HandleDeleteWebhook(c *gin.Context) {
	whID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid webhook id")
		return
	}

	if err := s.Store.DeleteWebhook(c.Request.Context(), whID); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to delete webhook")
		return
	}

	OK(c, http.StatusOK, "webhook deleted")
}
