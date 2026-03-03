package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/wzfukui/agent-native-im/internal/auth"
	"github.com/wzfukui/agent-native-im/internal/model"
)

// HandleAdminListUsers lists all entities with pagination.
func (s *Server) HandleAdminListUsers(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	entities, total, err := s.Store.ListAllEntities(c.Request.Context(), limit, offset)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to list entities")
		return
	}

	type entityWithOnline struct {
		*model.Entity
		Online bool `json:"online"`
	}

	result := make([]entityWithOnline, len(entities))
	for i, e := range entities {
		result[i] = entityWithOnline{Entity: e, Online: s.Hub.IsOnline(e.ID)}
	}

	OK(c, http.StatusOK, gin.H{
		"entities": result,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	})
}

// HandleAdminUpdateUser updates a user's display_name or status.
func (s *Server) HandleAdminUpdateUser(c *gin.Context) {
	entityID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid entity id")
		return
	}

	var req struct {
		DisplayName *string `json:"display_name"`
		Status      *string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	ctx := c.Request.Context()
	target, err := s.Store.GetEntityByID(ctx, entityID)
	if err != nil {
		Fail(c, http.StatusNotFound, "entity not found")
		return
	}

	if req.DisplayName != nil {
		target.DisplayName = *req.DisplayName
	}
	if req.Status != nil {
		target.Status = *req.Status
	}

	if err := s.Store.UpdateEntity(ctx, target); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to update entity")
		return
	}

	OK(c, http.StatusOK, target)
}

// HandleAdminDeleteUser soft-deletes an entity (sets status to disabled).
func (s *Server) HandleAdminDeleteUser(c *gin.Context) {
	entityID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid entity id")
		return
	}

	// Prevent self-deletion
	if entityID == auth.GetEntityID(c) {
		Fail(c, http.StatusBadRequest, "cannot delete yourself")
		return
	}

	if err := s.Store.DeleteEntity(c.Request.Context(), entityID); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to delete entity")
		return
	}

	OK(c, http.StatusOK, gin.H{"message": "entity deleted"})
}

// HandleAdminStats returns system statistics.
func (s *Server) HandleAdminStats(c *gin.Context) {
	stats, err := s.Store.GetSystemStats(c.Request.Context())
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to get stats")
		return
	}

	stats["ws_connections"] = s.Hub.ConnectionCount()

	OK(c, http.StatusOK, stats)
}

// HandleAdminListConversations lists all conversations with pagination.
func (s *Server) HandleAdminListConversations(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	convs, total, err := s.Store.ListAllConversations(c.Request.Context(), limit, offset)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to list conversations")
		return
	}

	OK(c, http.StatusOK, gin.H{
		"conversations": convs,
		"total":         total,
		"limit":         limit,
		"offset":        offset,
	})
}

// HandleAdminListAuditLogs returns audit log entries with filtering.
func (s *Server) HandleAdminListAuditLogs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	var entityID *int64
	if eidStr := c.Query("entity_id"); eidStr != "" {
		eid, err := strconv.ParseInt(eidStr, 10, 64)
		if err == nil {
			entityID = &eid
		}
	}
	action := c.Query("action")
	since := c.Query("since")

	logs, total, err := s.Store.ListAuditLogs(c.Request.Context(), entityID, action, since, limit, offset)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to list audit logs")
		return
	}

	OK(c, http.StatusOK, gin.H{
		"logs":   logs,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}
