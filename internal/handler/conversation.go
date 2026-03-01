package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/wzfukui/agent-native-im/internal/auth"
	"github.com/wzfukui/agent-native-im/internal/model"
)

type createConversationRequest struct {
	Title        string  `json:"title"`
	ConvType     string  `json:"conv_type"` // defaults to "direct"
	ParticipantIDs []int64 `json:"participant_ids"`
}

func (s *Server) HandleCreateConversation(c *gin.Context) {
	var req createConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	convType := model.ConvDirect
	if req.ConvType == "group" {
		convType = model.ConvGroup
	} else if req.ConvType == "channel" {
		convType = model.ConvChannel
	}

	conv := &model.Conversation{
		ConvType: convType,
		Title:    req.Title,
	}

	if err := s.Store.CreateConversation(ctx, conv); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to create conversation")
		return
	}

	// Add creator as owner participant
	if err := s.Store.AddParticipant(ctx, &model.Participant{
		ConversationID: conv.ID,
		EntityID:       entityID,
		Role:           model.RoleOwner,
	}); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to add creator as participant")
		return
	}

	// Add additional participants
	participantEntityIDs := []int64{entityID}
	for _, pid := range req.ParticipantIDs {
		if pid == entityID {
			continue
		}
		if err := s.Store.AddParticipant(ctx, &model.Participant{
			ConversationID: conv.ID,
			EntityID:       pid,
			Role:           model.RoleMember,
		}); err != nil {
			Fail(c, http.StatusInternalServerError, "failed to add participant")
			return
		}
		participantEntityIDs = append(participantEntityIDs, pid)
	}

	// Notify WebSocket hub
	if s.Hub != nil {
		s.Hub.NotifyNewConversation(conv.ID, participantEntityIDs...)
	}

	// Reload conversation with participants
	conv, err := s.Store.GetConversation(ctx, conv.ID)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to reload conversation")
		return
	}

	OK(c, http.StatusCreated, conv)
}

func (s *Server) HandleListConversations(c *gin.Context) {
	entityID := auth.GetEntityID(c)

	convs, err := s.Store.ListConversationsByEntity(c.Request.Context(), entityID)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to list conversations")
		return
	}

	OK(c, http.StatusOK, convs)
}

func (s *Server) HandleGetConversation(c *gin.Context) {
	convID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid conversation id")
		return
	}

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	// Verify participant
	ok, err := s.Store.IsParticipant(ctx, convID, entityID)
	if err != nil || !ok {
		Fail(c, http.StatusForbidden, "not a participant of this conversation")
		return
	}

	conv, err := s.Store.GetConversation(ctx, convID)
	if err != nil {
		Fail(c, http.StatusNotFound, "conversation not found")
		return
	}

	OK(c, http.StatusOK, conv)
}

// HandleAddParticipant adds an entity to a conversation.
func (s *Server) HandleAddParticipant(c *gin.Context) {
	convID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid conversation id")
		return
	}

	var req struct {
		EntityID int64  `json:"entity_id" binding:"required"`
		Role     string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "entity_id is required")
		return
	}

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	// Verify caller is participant
	ok, err := s.Store.IsParticipant(ctx, convID, entityID)
	if err != nil || !ok {
		Fail(c, http.StatusForbidden, "not a participant of this conversation")
		return
	}

	role := model.RoleMember
	if req.Role == "admin" {
		role = model.RoleAdmin
	} else if req.Role == "observer" {
		role = model.RoleObserver
	}

	if err := s.Store.AddParticipant(ctx, &model.Participant{
		ConversationID: convID,
		EntityID:       req.EntityID,
		Role:           role,
	}); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to add participant")
		return
	}

	if s.Hub != nil {
		s.Hub.NotifyNewConversation(convID, req.EntityID)
	}

	OK(c, http.StatusCreated, "participant added")
}

// HandleRemoveParticipant removes an entity from a conversation.
func (s *Server) HandleRemoveParticipant(c *gin.Context) {
	convID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid conversation id")
		return
	}

	targetID, err := strconv.ParseInt(c.Param("entityId"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid entity id")
		return
	}

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	// Verify caller is participant
	ok, err := s.Store.IsParticipant(ctx, convID, entityID)
	if err != nil || !ok {
		Fail(c, http.StatusForbidden, "not a participant of this conversation")
		return
	}

	if err := s.Store.RemoveParticipant(ctx, convID, targetID); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to remove participant")
		return
	}

	OK(c, http.StatusOK, "participant removed")
}
