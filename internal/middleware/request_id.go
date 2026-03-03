package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/gin-gonic/gin"
)

const RequestIDKey = "request_id"

// RequestID generates a unique request ID and stores it in the context.
// Format: "req_" + 12 hex chars (6 random bytes) + "_" + unix millis suffix.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		b := make([]byte, 6)
		_, _ = rand.Read(b)
		rid := "req_" + hex.EncodeToString(b) + "_" + formatMillis(time.Now())
		c.Set(RequestIDKey, rid)
		c.Header("X-Request-ID", rid)
		c.Next()
	}
}

func formatMillis(t time.Time) string {
	ms := t.UnixMilli() % 1_000_000 // last 6 digits for brevity
	buf := make([]byte, 0, 6)
	s := uint64(ms)
	for i := 0; i < 6; i++ {
		buf = append(buf, byte('0'+s%10))
		s /= 10
	}
	// reverse
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}

// GetRequestID extracts the request ID from a Gin context.
func GetRequestID(c *gin.Context) string {
	if rid, ok := c.Get(RequestIDKey); ok {
		return rid.(string)
	}
	return ""
}
