package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/wzfukui/agent-native-im/internal/auth"
	"github.com/wzfukui/agent-native-im/internal/model"
)

// GET /conversations/:id/memories
func (s *Server) HandleListMemories(c *gin.Context) {
	convID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid conversation id")
		return
	}

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	ok, _ := s.Store.IsParticipant(ctx, convID, entityID)
	if !ok {
		FailWithCode(c, http.StatusForbidden, ErrCodePermNotParticipant, "not a participant")
		return
	}

	mems, err := s.Store.ListMemories(ctx, convID)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to list memories")
		return
	}
	if mems == nil {
		mems = []*model.ConversationMemory{}
	}

	// Also return conversation prompt
	conv, _ := s.Store.GetConversation(ctx, convID)
	prompt := ""
	if conv != nil {
		prompt = conv.Prompt
	}

	OK(c, http.StatusOK, gin.H{
		"memories": mems,
		"prompt":   prompt,
	})
}

// POST /conversations/:id/memories
func (s *Server) HandleUpsertMemory(c *gin.Context) {
	convID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid conversation id")
		return
	}

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	ok, _ := s.Store.IsParticipant(ctx, convID, entityID)
	if !ok {
		FailWithCode(c, http.StatusForbidden, ErrCodePermNotParticipant, "not a participant")
		return
	}

	var req struct {
		Key     string `json:"key" binding:"required"`
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "key and content are required")
		return
	}

	mem := &model.ConversationMemory{
		ConversationID: convID,
		Key:            req.Key,
		Content:        req.Content,
		UpdatedBy:      entityID,
	}

	if err := s.Store.UpsertMemory(ctx, mem); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to save memory")
		return
	}

	// Broadcast
	if s.Hub != nil {
		s.Hub.BroadcastEvent(convID, "conversation.memory_updated", map[string]interface{}{
			"conversation_id": convID,
			"key":             req.Key,
			"updated_by":      entityID,
		})
	}

	OK(c, http.StatusOK, mem)
}

// DELETE /conversations/:id/memories/:memId
func (s *Server) HandleDeleteMemory(c *gin.Context) {
	convID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid conversation id")
		return
	}

	memID, err := strconv.ParseInt(c.Param("memId"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid memory id")
		return
	}

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	ok, _ := s.Store.IsParticipant(ctx, convID, entityID)
	if !ok {
		FailWithCode(c, http.StatusForbidden, ErrCodePermNotParticipant, "not a participant")
		return
	}

	if err := s.Store.DeleteMemory(ctx, memID); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to delete memory")
		return
	}

	// Broadcast
	if s.Hub != nil {
		s.Hub.BroadcastEvent(convID, "conversation.memory_updated", map[string]interface{}{
			"conversation_id": convID,
			"action":          "deleted",
			"memory_id":       memID,
		})
	}

	OK(c, http.StatusOK, nil)
}
