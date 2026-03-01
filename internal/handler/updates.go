package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wzfukui/agent-native-im/internal/auth"
)

func (s *Server) HandleUpdates(c *gin.Context) {
	entityID := auth.GetEntityID(c)
	offset, _ := strconv.ParseInt(c.DefaultQuery("offset", "0"), 10, 64)
	timeout, _ := strconv.Atoi(c.DefaultQuery("timeout", "30"))
	if timeout > 60 {
		timeout = 60
	}
	if timeout < 1 {
		timeout = 1
	}

	ctx := c.Request.Context()

	// Check for existing messages first
	msgs, err := s.Store.GetUpdatesForEntity(ctx, entityID, offset)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to get updates")
		return
	}
	if len(msgs) > 0 {
		OK(c, http.StatusOK, msgs)
		return
	}

	// No messages yet, wait
	ch := s.Hub.RegisterWaiter(entityID)
	defer s.Hub.UnregisterWaiter(entityID, ch)

	select {
	case <-ch:
		msgs, err = s.Store.GetUpdatesForEntity(ctx, entityID, offset)
		if err != nil {
			Fail(c, http.StatusInternalServerError, "failed to get updates")
			return
		}
		OK(c, http.StatusOK, msgs)

	case <-time.After(time.Duration(timeout) * time.Second):
		OK(c, http.StatusOK, []interface{}{})

	case <-ctx.Done():
		// client disconnected
	}
}
