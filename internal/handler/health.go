package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

var startTime = time.Now()

// HandleHealthCheck returns system health status including DB connectivity,
// connection pool stats, WebSocket hub stats, and server uptime.
func (s *Server) HandleHealthCheck(c *gin.Context) {
	ctx := c.Request.Context()

	// Check database connectivity
	dbStatus := "ok"
	if _, err := s.Store.GetSystemStats(ctx); err != nil {
		dbStatus = "error"
	}

	// Gather connection pool stats
	poolStats := s.Store.DBPoolStats()

	// Gather WebSocket hub stats
	wsConnections := 0
	wsConversations := 0
	if s.Hub != nil {
		wsConnections = s.Hub.ConnectionCount()
		wsConversations = s.Hub.ConversationCount()
	}

	uptime := time.Since(startTime)

	OK(c, http.StatusOK, gin.H{
		"status": "ok",
		"db": gin.H{
			"status": dbStatus,
			"pool": gin.H{
				"max_open":            poolStats.MaxOpenConnections,
				"open":                poolStats.OpenConnections,
				"in_use":              poolStats.InUse,
				"idle":                poolStats.Idle,
				"wait_count":          poolStats.WaitCount,
				"wait_duration_ms":    poolStats.WaitDuration.Milliseconds(),
				"max_idle_closed":     poolStats.MaxIdleClosed,
				"max_lifetime_closed": poolStats.MaxLifetimeClosed,
			},
		},
		"ws": gin.H{
			"connections":   wsConnections,
			"conversations": wsConversations,
		},
		"uptime_seconds": int(uptime.Seconds()),
		"uptime":         uptime.String(),
	})
}
