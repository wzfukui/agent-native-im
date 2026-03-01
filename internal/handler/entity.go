package handler

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wzfukui/agent-native-im/internal/auth"
	"github.com/wzfukui/agent-native-im/internal/model"
)

type createEntityRequest struct {
	Name        string `json:"name" binding:"required"`
	DisplayName string `json:"display_name"`
	EntityType  string `json:"entity_type"` // defaults to "bot"
}

// HandleCreateEntity creates a bot/service entity owned by the authenticated user.
func (s *Server) HandleCreateEntity(c *gin.Context) {
	if auth.GetEntityType(c) != model.EntityUser {
		Fail(c, http.StatusForbidden, "only users can create entities")
		return
	}

	var req createEntityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "name is required")
		return
	}

	entityType := model.EntityBot
	if req.EntityType == "service" {
		entityType = model.EntityService
	}

	ownerID := auth.GetEntityID(c)
	displayName := req.DisplayName
	if displayName == "" {
		displayName = req.Name
	}

	entity := &model.Entity{
		EntityType:  entityType,
		Name:        req.Name,
		DisplayName: displayName,
		Status:      "active",
		OwnerID:     &ownerID,
	}

	if err := s.Store.CreateEntity(c.Request.Context(), entity); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to create entity")
		return
	}

	// Generate API key credential
	apiKey := uuid.New().String()
	apiKeyHash := fmt.Sprintf("%x", sha256.Sum256([]byte(apiKey)))

	cred := &model.Credential{
		EntityID:   entity.ID,
		CredType:   model.CredAPIKey,
		SecretHash: apiKeyHash,
		RawPrefix:  apiKey[:8],
	}

	if err := s.Store.CreateCredential(c.Request.Context(), cred); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to create credential")
		return
	}

	OK(c, http.StatusCreated, gin.H{
		"entity":  entity,
		"api_key": apiKey, // shown only once
	})
}

// HandleListEntities lists entities owned by the authenticated user.
func (s *Server) HandleListEntities(c *gin.Context) {
	if auth.GetEntityType(c) != model.EntityUser {
		Fail(c, http.StatusForbidden, "only users can list owned entities")
		return
	}

	entities, err := s.Store.ListEntitiesByOwner(c.Request.Context(), auth.GetEntityID(c))
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to list entities")
		return
	}

	OK(c, http.StatusOK, entities)
}

// HandleDeleteEntity soft-deletes an entity owned by the authenticated user.
func (s *Server) HandleDeleteEntity(c *gin.Context) {
	if auth.GetEntityType(c) != model.EntityUser {
		Fail(c, http.StatusForbidden, "only users can delete entities")
		return
	}

	entityID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid entity id")
		return
	}

	// Verify ownership
	target, err := s.Store.GetEntityByID(c.Request.Context(), entityID)
	if err != nil {
		Fail(c, http.StatusNotFound, "entity not found")
		return
	}

	if target.OwnerID == nil || *target.OwnerID != auth.GetEntityID(c) {
		Fail(c, http.StatusForbidden, "not the owner of this entity")
		return
	}

	if err := s.Store.DeleteEntity(c.Request.Context(), entityID); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to delete entity")
		return
	}

	OK(c, http.StatusOK, "entity deleted")
}
