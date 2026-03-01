package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/wzfukui/agent-native-im/internal/auth"
	"github.com/wzfukui/agent-native-im/internal/config"
	"github.com/wzfukui/agent-native-im/internal/model"
	"github.com/wzfukui/agent-native-im/internal/store"
	"github.com/wzfukui/agent-native-im/internal/webhook"
	"github.com/wzfukui/agent-native-im/internal/ws"
)

type AuthHelper struct {
	Secret string
}

func (a *AuthHelper) GenerateToken(entityID int64, entityType model.EntityType) (string, error) {
	return auth.GenerateToken(a.Secret, entityID, entityType)
}

type Server struct {
	Config  *config.Config
	Store   store.Store
	Hub     *ws.Hub
	Webhook *webhook.Deliverer
	Auth    *AuthHelper
}

func NewRouter(s *Server) *gin.Engine {
	r := gin.Default()
	r.Use(corsMiddleware())

	v1 := r.Group("/api/v1")
	{
		// Public
		v1.GET("/ping", HandlePing)
		v1.POST("/auth/login", s.HandleLogin)

		// Authenticated (any entity type)
		authed := v1.Group("")
		authed.Use(auth.EntityAuth(s.Config.JWTSecret, s.Store))
		{
			// Entity self-info
			authed.GET("/me", s.HandleMe)

			// Entity management (user-only at handler level)
			authed.POST("/entities", s.HandleCreateEntity)
			authed.GET("/entities", s.HandleListEntities)
			authed.DELETE("/entities/:id", s.HandleDeleteEntity)

			// Webhook management
			authed.POST("/webhooks", s.HandleCreateWebhook)
			authed.GET("/webhooks", s.HandleListWebhooks)
			authed.DELETE("/webhooks/:id", s.HandleDeleteWebhook)

			// Conversations
			authed.POST("/conversations", s.HandleCreateConversation)
			authed.GET("/conversations", s.HandleListConversations)
			authed.GET("/conversations/:id", s.HandleGetConversation)

			// Participants
			authed.POST("/conversations/:id/participants", s.HandleAddParticipant)
			authed.DELETE("/conversations/:id/participants/:entityId", s.HandleRemoveParticipant)

			// Messages
			authed.POST("/messages/send", s.HandleSendMessage)
			authed.GET("/conversations/:id/messages", s.HandleListMessages)

			// Long polling
			authed.GET("/updates", s.HandleUpdates)
		}

		// WebSocket (auth via query param)
		v1.GET("/ws", s.HandleWS)
	}

	return r
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			origin = "*"
		}

		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Max-Age", "86400")

		if strings.ToUpper(c.Request.Method) == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
