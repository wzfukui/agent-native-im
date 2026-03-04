package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiter implements a simple token bucket algorithm for rate limiting
type RateLimiter struct {
	visitors map[string]*visitor
	mu       sync.RWMutex
	rate     int           // requests per interval
	interval time.Duration // time interval
	cleanup  time.Duration // cleanup interval for old visitors
}

type visitor struct {
	tokens    int
	lastVisit time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(rate int, interval time.Duration) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		rate:     rate,
		interval: interval,
		cleanup:  interval * 10, // cleanup after 10 intervals
	}

	// Start cleanup goroutine
	go rl.cleanupVisitors()

	return rl
}

// cleanupVisitors removes old visitor entries
func (rl *RateLimiter) cleanupVisitors() {
	ticker := time.NewTicker(rl.cleanup)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, v := range rl.visitors {
			if now.Sub(v.lastVisit) > rl.cleanup {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// getVisitor returns the visitor for the given IP, creating if necessary
func (rl *RateLimiter) getVisitor(ip string) *visitor {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[ip]
	if !exists {
		v = &visitor{
			tokens:    rl.rate,
			lastVisit: time.Now(),
		}
		rl.visitors[ip] = v
	}

	// Refill tokens based on time elapsed
	now := time.Now()
	elapsed := now.Sub(v.lastVisit)
	tokensToAdd := int(elapsed / rl.interval)
	if tokensToAdd > 0 {
		v.tokens = min(v.tokens+tokensToAdd, rl.rate)
		v.lastVisit = now
	}

	return v
}

// Middleware returns a Gin middleware handler for rate limiting
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip rate limiting in test mode
		if gin.Mode() == gin.TestMode {
			c.Next()
			return
		}

		ip := c.ClientIP()
		v := rl.getVisitor(ip)

		rl.mu.Lock()
		if v.tokens <= 0 {
			rl.mu.Unlock()
			c.JSON(http.StatusTooManyRequests, gin.H{
				"ok": false,
				"error": map[string]interface{}{
					"code":       "RATE_LIMIT_EXCEEDED",
					"message":    fmt.Sprintf("Rate limit exceeded. Max %d requests per %v", rl.rate, rl.interval),
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

// CreateRateLimiters creates rate limiters for different endpoints
func CreateRateLimiters() map[string]*RateLimiter {
	return map[string]*RateLimiter{
		"auth":     NewRateLimiter(5, time.Minute),    // 5 requests per minute for auth
		"login":    NewRateLimiter(3, time.Minute),    // 3 login attempts per minute
		"register": NewRateLimiter(2, time.Minute),    // 2 registrations per minute
		"api":      NewRateLimiter(60, time.Minute),   // 60 API requests per minute
		"message":  NewRateLimiter(30, time.Minute),   // 30 messages per minute
		"file":     NewRateLimiter(10, time.Minute),   // 10 file uploads per minute
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}