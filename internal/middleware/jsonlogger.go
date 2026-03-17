package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// JSONLogger returns a GIN middleware that writes structured access logs via slog.
func JSONLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		latency := time.Since(start)

		var entityID int64
		if eid, exists := c.Get("entityID"); exists {
			if id, ok := eid.(int64); ok {
				entityID = id
			}
		}

		attrs := []any{
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", latency.Milliseconds(),
			"client_ip", c.ClientIP(),
		}
		if entityID > 0 {
			attrs = append(attrs, "entity_id", entityID)
		}
		if rid := GetRequestID(c); rid != "" {
			attrs = append(attrs, "request_id", rid)
		}

		status := c.Writer.Status()
		switch {
		case status >= 500:
			slog.Error("HTTP request", attrs...)
		case status >= 400:
			slog.Warn("HTTP request", attrs...)
		default:
			slog.Info("HTTP request", attrs...)
		}
	}
}
