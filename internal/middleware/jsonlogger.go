package middleware

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
)

type accessLogEntry struct {
	Time      string `json:"time"`
	Level     string `json:"level"`
	Method    string `json:"method"`
	Path      string `json:"path"`
	Status    int    `json:"status"`
	LatencyMs int64  `json:"latency_ms"`
	EntityID  int64  `json:"entity_id,omitempty"`
	ClientIP  string `json:"client_ip"`
	RequestID string `json:"request_id,omitempty"`
}

// JSONLogger returns a GIN middleware that writes structured JSON access logs to stdout.
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

		entry := accessLogEntry{
			Time:      start.UTC().Format(time.RFC3339),
			Level:     "info",
			Method:    c.Request.Method,
			Path:      c.Request.URL.Path,
			Status:    c.Writer.Status(),
			LatencyMs: latency.Milliseconds(),
			EntityID:  entityID,
			ClientIP:  c.ClientIP(),
			RequestID: GetRequestID(c),
		}

		if c.Writer.Status() >= 500 {
			entry.Level = "error"
		} else if c.Writer.Status() >= 400 {
			entry.Level = "warn"
		}

		data, err := json.Marshal(entry)
		if err != nil {
			return
		}
		fmt.Println(string(data))
	}
}
