package middleware

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wzfukui/agent-native-im/internal/auth"
	"github.com/wzfukui/agent-native-im/internal/model"
	"github.com/wzfukui/agent-native-im/internal/store"
)

// Audit logs each request with entity, method, path, status, and latency.
// It persists audit entries to the database when a store is provided.
func Audit(s ...store.Store) gin.HandlerFunc {
	var auditStore store.Store
	if len(s) > 0 {
		auditStore = s[0]
	}

	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start).Milliseconds()
		entityID := auth.GetEntityID(c)

		slog.Info("AUDIT",
			"entity_id", entityID, "method", c.Request.Method, "path", c.Request.URL.Path, "status", c.Writer.Status(), "latency_ms", latency)

		// Persist to database (best-effort, non-blocking)
		if auditStore != nil {
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				details, _ := json.Marshal(map[string]interface{}{
					"method":  c.Request.Method,
					"path":    c.Request.URL.Path,
					"status":  c.Writer.Status(),
					"latency": latency,
				})
				var eid *int64
				if entityID > 0 {
					eid = &entityID
				}
				action := buildAuditAction(c.Request.Method, c.Request.URL.Path)
				entry := &model.AuditLog{
					EntityID:  eid,
					Action:    action,
					Details:   details,
					IPAddress: c.ClientIP(),
				}
				if err := auditStore.CreateAuditLog(ctx, entry); err != nil {
					slog.Error("audit: failed to persist audit log", "error", err)
				}
			}()
		}
	}
}

func buildAuditAction(method, path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return method
	}
	action := method + " " + path
	if len(action) <= 50 {
		return action
	}
	trimmedPath := path
	if len(trimmedPath) > 40 {
		trimmedPath = "..." + trimmedPath[len(trimmedPath)-37:]
	}
	action = method + " " + trimmedPath
	if len(action) > 50 {
		return action[:50]
	}
	return action
}
