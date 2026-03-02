package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wzfukui/agent-native-im/internal/auth"
)

// Audit logs each request with entity, method, path, status, and latency.
func Audit() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start).Milliseconds()
		entityID := auth.GetEntityID(c)
		log.Printf("AUDIT entity=%d method=%s path=%s status=%d latency=%dms",
			entityID, c.Request.Method, c.Request.URL.Path, c.Writer.Status(), latency)
	}
}
