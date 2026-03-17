package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wzfukui/agent-native-im/internal/model"
)

// RateLimiter implements a simple token bucket algorithm for rate limiting
type RateLimiter struct {
	visitors map[string]*visitor
	mu       sync.RWMutex
	rate     int           // default requests per interval
	apiRate  int           // requests per interval for API-key authenticated entities (0 = use default)
	interval time.Duration // time interval
	cleanup  time.Duration // cleanup interval for old visitors
}

type visitor struct {
	tokens    int
	maxTokens int // per-visitor cap (may differ for API-key vs IP-based)
	lastVisit time.Time
}

// NewRateLimiter creates a new rate limiter with the given default rate.
func NewRateLimiter(rate int, interval time.Duration) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		rate:     rate,
		interval: interval,
		cleanup:  time.Hour, // clean up entries not seen for >1 hour
	}

	// Start cleanup goroutine
	go rl.cleanupVisitors()

	return rl
}

// WithAPIRate sets a separate (typically higher) rate limit for API-key
// authenticated bot/service entities. Returns the same RateLimiter for chaining.
func (rl *RateLimiter) WithAPIRate(apiRate int) *RateLimiter {
	rl.apiRate = apiRate
	return rl
}

// cleanupVisitors removes entries not seen for more than the cleanup duration.
func (rl *RateLimiter) cleanupVisitors() {
	ticker := time.NewTicker(10 * time.Minute) // check every 10 minutes
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for key, v := range rl.visitors {
			if now.Sub(v.lastVisit) > rl.cleanup {
				delete(rl.visitors, key)
			}
		}
		rl.mu.Unlock()
	}
}

// getVisitor returns the visitor for the given key, creating if necessary.
// effectiveRate is the max tokens for this particular visitor.
func (rl *RateLimiter) getVisitor(key string, effectiveRate int) *visitor {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[key]
	if !exists {
		v = &visitor{
			tokens:    effectiveRate,
			maxTokens: effectiveRate,
			lastVisit: time.Now(),
		}
		rl.visitors[key] = v
		return v
	}

	// If the effective rate changed (e.g., entity got upgraded), adjust the cap.
	if v.maxTokens != effectiveRate {
		v.maxTokens = effectiveRate
	}

	// Refill tokens based on time elapsed
	now := time.Now()
	elapsed := now.Sub(v.lastVisit)
	tokensToAdd := int(elapsed / rl.interval)
	if tokensToAdd > 0 {
		v.tokens = min(v.tokens+tokensToAdd, v.maxTokens)
		v.lastVisit = now
	}

	return v
}

// rateLimitKey returns the entity ID if authenticated, otherwise the client IP.
func rateLimitKey(c *gin.Context) string {
	if eid, exists := c.Get("entityID"); exists {
		if id, ok := eid.(int64); ok && id > 0 {
			return fmt.Sprintf("entity:%d", id)
		}
	}
	return c.ClientIP()
}

// isAPIKeyEntity returns true if the request is from an authenticated bot or service entity.
func isAPIKeyEntity(c *gin.Context) bool {
	v, exists := c.Get("entityType")
	if !exists {
		return false
	}
	et, ok := v.(model.EntityType)
	if !ok {
		return false
	}
	return et == model.EntityBot || et == model.EntityService
}

// Middleware returns a Gin middleware handler for rate limiting.
// If apiRate is configured and the request comes from an authenticated
// bot/service entity, the higher apiRate is applied.
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip rate limiting in test mode
		if gin.Mode() == gin.TestMode {
			c.Next()
			return
		}

		key := rateLimitKey(c)

		// Determine effective rate: use apiRate for bot/service entities if set.
		effectiveRate := rl.rate
		if rl.apiRate > 0 && isAPIKeyEntity(c) {
			effectiveRate = rl.apiRate
		}

		v := rl.getVisitor(key, effectiveRate)

		rl.mu.Lock()
		if v.tokens <= 0 {
			rl.mu.Unlock()
			c.JSON(http.StatusTooManyRequests, gin.H{
				"ok": false,
				"error": map[string]interface{}{
					"code":       "RATE_LIMIT_EXCEEDED",
					"message":    fmt.Sprintf("Rate limit exceeded. Max %d requests per %v", effectiveRate, rl.interval),
					"status":     429,
					"request_id": c.GetString("request_id"),
					"details": map[string]interface{}{
						"retry_after": int(rl.interval.Seconds()),
					},
				},
			})
			c.Abort()
			return
		}
		v.tokens--
		rl.mu.Unlock()

		c.Next()
	}
}

// CreateRateLimiters creates rate limiters for different endpoints.
func CreateRateLimiters() map[string]*RateLimiter {
	return map[string]*RateLimiter{
		"auth":     NewRateLimiter(5, time.Minute),                                  // 5 requests per minute for auth
		"login":    NewRateLimiter(3, time.Minute),                                  // 3 login attempts per minute
		"register": NewRateLimiter(2, time.Minute),                                  // 2 registrations per minute
		"api":      NewRateLimiter(60, time.Minute).WithAPIRate(120),                // 60/min default, 120/min for bots
		"message":  NewRateLimiter(30, time.Minute).WithAPIRate(60),                 // 30/min default, 60/min for bots
		"file":     NewRateLimiter(10, time.Minute).WithAPIRate(20),                 // 10/min default, 20/min for bots
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
