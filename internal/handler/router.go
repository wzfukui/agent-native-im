package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/wzfukui/agent-native-im/internal/auth"
	"github.com/wzfukui/agent-native-im/internal/config"
	"github.com/wzfukui/agent-native-im/internal/middleware"
	"github.com/wzfukui/agent-native-im/internal/filestore"
	"github.com/wzfukui/agent-native-im/internal/model"
	"github.com/wzfukui/agent-native-im/internal/push"
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
	Config    *config.Config
	Store     store.Store
	Hub       *ws.Hub
	Webhook   *webhook.Deliverer
	Auth      *AuthHelper
	FileStore filestore.FileStore
	Push      *push.Sender
}

func NewRouter(s *Server) *gin.Engine {
	r := gin.Default()
	r.Use(corsMiddleware())

	v1 := r.Group("/api/v1")
	{
		// Public
		v1.GET("/ping", HandlePing)
		v1.GET("/skill-template", HandleSkillTemplate)
		v1.POST("/auth/login", s.HandleLogin)
		v1.POST("/auth/register", s.HandleRegister)

		// Public push key endpoint (no auth needed)
		v1.GET("/push/vapid-key", s.HandleGetVAPIDKey)

		// Authenticated (any entity type, including bootstrap keys)
		authed := v1.Group("")
		authed.Use(auth.EntityAuth(s.Config.JWTSecret, s.Store))
		authed.Use(middleware.Audit(s.Store))
		{
			// Bootstrap-key-accessible endpoints
			authed.GET("/me", s.HandleMe)
			authed.POST("/auth/refresh", s.HandleRefreshToken)

			// Full-auth-only endpoints (bootstrap keys blocked)
			full := authed.Group("")
			full.Use(auth.RequireFullAuth())
			{
				// User management
				full.PUT("/me", s.HandleUpdateProfile)
				full.PUT("/me/password", s.HandleChangePassword)

				full.GET("/me/devices", s.HandleListDevices)
				full.DELETE("/me/devices/:deviceId", s.HandleKickDevice)

			// Admin-only endpoints
				admin := full.Group("")
				admin.Use(auth.RequireAdmin(s.Store, s.Config.AdminUser))
				{
					admin.POST("/admin/users", s.HandleCreateUser)
					admin.GET("/admin/users", s.HandleAdminListUsers)
					admin.PUT("/admin/users/:id", s.HandleAdminUpdateUser)
					admin.DELETE("/admin/users/:id", s.HandleAdminDeleteUser)
					admin.GET("/admin/stats", s.HandleAdminStats)
					admin.GET("/admin/conversations", s.HandleAdminListConversations)
					admin.GET("/admin/audit-logs", s.HandleAdminListAuditLogs)
				}
				// Entity management (user-only at handler level)
				full.POST("/entities", s.HandleCreateEntity)
				full.GET("/entities", s.HandleListEntities)
				full.PUT("/entities/:id", s.HandleUpdateEntity)
				full.DELETE("/entities/:id", s.HandleDeleteEntity)
				full.POST("/entities/:id/approve", s.HandleApproveConnection)
				full.GET("/entities/:id/status", s.HandleEntityStatus)
				full.GET("/entities/:id/credentials", s.HandleGetCredentials)
				full.POST("/entities/:id/reactivate", s.HandleReactivateEntity)
				full.POST("/presence/batch", s.HandleBatchPresence)

				// Webhook management
				full.POST("/webhooks", s.HandleCreateWebhook)
				full.GET("/webhooks", s.HandleListWebhooks)
				full.DELETE("/webhooks/:id", s.HandleDeleteWebhook)

				// Conversations
				full.POST("/conversations", s.HandleCreateConversation)
				full.GET("/conversations", s.HandleListConversations)
				full.GET("/conversations/:id", s.HandleGetConversation)
			full.PUT("/conversations/:id", s.HandleUpdateConversation)

				// Participants & lifecycle
				full.POST("/conversations/:id/participants", s.HandleAddParticipant)
				full.DELETE("/conversations/:id/participants/:entityId", s.HandleRemoveParticipant)
				full.PUT("/conversations/:id/subscription", s.HandleUpdateSubscription)
				full.POST("/conversations/:id/read", s.HandleMarkAsRead)
				full.POST("/conversations/:id/leave", s.HandleLeaveConversation)
				full.POST("/conversations/:id/archive", s.HandleArchiveConversation)
				full.POST("/conversations/:id/unarchive", s.HandleUnarchiveConversation)

				// Messages
				full.POST("/messages/send", s.HandleSendMessage)
				full.DELETE("/messages/:id", s.HandleRevokeMessage)
				full.PUT("/messages/:id", s.HandleEditMessage)
				full.POST("/messages/:id/respond", s.HandleInteractionResponse)
				full.GET("/conversations/:id/messages", s.HandleListMessages)
				full.GET("/conversations/:id/search", s.HandleSearchMessages)

				// Invite links
				full.POST("/conversations/:id/invite", s.HandleCreateInviteLink)
				full.GET("/conversations/:id/invites", s.HandleListInviteLinks)
				full.DELETE("/invites/:id", s.HandleDeleteInviteLink)

				// File upload
				full.POST("/files/upload", s.HandleFileUpload)

				// Push notifications
				full.POST("/push/subscribe", s.HandleRegisterPush)
				full.POST("/push/unsubscribe", s.HandleUnregisterPush)

				// Invite join
				full.GET("/invite/:code", s.HandleGetInviteInfo)
				full.POST("/invite/:code/join", s.HandleJoinViaInvite)

				// Tasks
				full.POST("/conversations/:id/tasks", s.HandleCreateTask)
				full.GET("/conversations/:id/tasks", s.HandleListTasks)
				full.GET("/tasks/:taskId", s.HandleGetTask)
				full.PUT("/tasks/:taskId", s.HandleUpdateTask)
				full.DELETE("/tasks/:taskId", s.HandleDeleteTask)

				// Memories
				full.GET("/conversations/:id/memories", s.HandleListMemories)
				full.POST("/conversations/:id/memories", s.HandleUpsertMemory)
				full.DELETE("/conversations/:id/memories/:memId", s.HandleDeleteMemory)

				// Change Requests
				full.POST("/conversations/:id/change-requests", s.HandleCreateChangeRequest)
				full.GET("/conversations/:id/change-requests", s.HandleListChangeRequests)
				full.POST("/conversations/:id/change-requests/:reqId/approve", s.HandleApproveChangeRequest)
				full.POST("/conversations/:id/change-requests/:reqId/reject", s.HandleRejectChangeRequest)

				// Long polling
				full.GET("/updates", s.HandleUpdates)
			}
		}

		// WebSocket (auth via query param, supports bootstrap keys)
		v1.GET("/ws", s.HandleWS)

		// Static file serving for uploads
		if s.FileStore != nil {
			r.Static("/files", s.FileStore.ServePath())
		}
	}

	return r
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
		c.Header("Access-Control-Max-Age", "86400")

		if origin != "" {
			// Specific origin: allow credentials
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
		} else {
			// No origin: wildcard without credentials
			c.Header("Access-Control-Allow-Origin", "*")
		}

		if strings.ToUpper(c.Request.Method) == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
