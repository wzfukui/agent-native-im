package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

var startTime = time.Now()

// HandleHealthCheck returns system health status including DB connectivity,
// WebSocket hub stats, and server uptime.
func (s *Server) HandleHealthCheck(c *gin.Context) {
	ctx := c.Request.Context()

	// Check database connectivity
	dbStatus := "ok"
	if _, err := s.Store.GetSystemStats(ctx); err != nil {
		dbStatus = "error"
	}

	// Gather WebSocket hub stats
	wsConnections := 0
	wsConversations := 0
	if s.Hub != nil {
		wsConnections = s.Hub.ConnectionCount()
		wsConversations = s.Hub.ConversationCount()
	}

	uptime := time.Since(startTime)

	OK(c, http.StatusOK, gin.H{
		"status":           "ok",
		"db":               dbStatus,
		"ws_connections":   wsConnections,
		"ws_conversations": wsConversations,
		"uptime_seconds":   int(uptime.Seconds()),
		"uptime":           uptime.String(),
	})
}
