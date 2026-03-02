package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/wzfukui/agent-native-im/internal/auth"
	"github.com/wzfukui/agent-native-im/internal/model"
)

// HandleRegisterPush registers a Web Push subscription for the authenticated entity.
func (s *Server) HandleRegisterPush(c *gin.Context) {
	var req struct {
		Endpoint  string `json:"endpoint" binding:"required"`
		KeyP256DH string `json:"key_p256dh" binding:"required"`
		KeyAuth   string `json:"key_auth" binding:"required"`
		DeviceID  string `json:"device_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "endpoint, key_p256dh, and key_auth are required")
		return
	}

	entityID := auth.GetEntityID(c)
	sub := &model.PushSubscription{
		EntityID:  entityID,
		DeviceID:  req.DeviceID,
		Endpoint:  req.Endpoint,
		KeyP256DH: req.KeyP256DH,
		KeyAuth:   req.KeyAuth,
	}

	if err := s.Store.CreatePushSubscription(c.Request.Context(), sub); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to register push subscription")
		return
	}

	OK(c, http.StatusCreated, gin.H{"message": "push subscription registered"})
}

// HandleUnregisterPush removes a Web Push subscription.
func (s *Server) HandleUnregisterPush(c *gin.Context) {
	var req struct {
		Endpoint string `json:"endpoint" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "endpoint is required")
		return
	}

	entityID := auth.GetEntityID(c)
	_ = s.Store.DeletePushSubscription(c.Request.Context(), entityID, req.Endpoint)
	OK(c, http.StatusOK, gin.H{"message": "push subscription removed"})
}

// HandleGetVAPIDKey returns the VAPID public key for push subscription.
func (s *Server) HandleGetVAPIDKey(c *gin.Context) {
	if s.Config.VAPIDPublicKey == "" {
		Fail(c, http.StatusNotFound, "push notifications not configured")
		return
	}
	OK(c, http.StatusOK, gin.H{"public_key": s.Config.VAPIDPublicKey})
}
