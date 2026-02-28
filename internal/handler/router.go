package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/wzfukui/agent-native-im/internal/auth"
	"github.com/wzfukui/agent-native-im/internal/config"
	"github.com/wzfukui/agent-native-im/internal/store"
	"github.com/wzfukui/agent-native-im/internal/ws"
)

type AuthHelper struct {
	Secret string
}

func (a *AuthHelper) GenerateToken(userID int64, username string) (string, error) {
	return auth.GenerateToken(a.Secret, userID, username)
}

type Server struct {
	Config *config.Config
	Store  *store.Store
	Hub    *ws.Hub
	Auth   *AuthHelper
}

func NewRouter(s *Server) *gin.Engine {
	r := gin.Default()
	r.Use(corsMiddleware())

	v1 := r.Group("/api/v1")
	{
		// Public
		v1.GET("/ping", HandlePing)
		v1.POST("/auth/login", s.HandleLogin)

		// User-authenticated
		userRoutes := v1.Group("")
		userRoutes.Use(auth.UserAuth(s.Config.JWTSecret))
		{
			userRoutes.POST("/bots", s.HandleCreateBot)
			userRoutes.GET("/bots", s.HandleListBots)
			userRoutes.DELETE("/bots/:id", s.HandleDeleteBot)
		}

		// Any-authenticated (user or bot)
		anyRoutes := v1.Group("")
		anyRoutes.Use(auth.AnyAuth(s.Config.JWTSecret, s.Store))
		{
			anyRoutes.GET("/conversations", s.HandleListConversations)
			anyRoutes.POST("/conversations", s.HandleCreateConversation)
			anyRoutes.GET("/conversations/:id", s.HandleGetConversation)
			anyRoutes.GET("/conversations/:id/messages", s.HandleListMessages)
			anyRoutes.POST("/messages/send", s.HandleSendMessage)
		}

		// Bot-authenticated
		botRoutes := v1.Group("")
		botRoutes.Use(auth.BotAuth(s.Store))
		{
			botRoutes.GET("/bot/me", s.HandleBotMe)
			botRoutes.GET("/updates", s.HandleUpdates)
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
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
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
