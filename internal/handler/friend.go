package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/wzfukui/agent-native-im/internal/auth"
	"github.com/wzfukui/agent-native-im/internal/model"
)

func orderedFriendEntityPair(entityA, entityB int64) (int64, int64) {
	if entityA < entityB {
		return entityA, entityB
	}
	return entityB, entityA
}

func (s *Server) resolveActingEntity(c *gin.Context, requestedID int64) (*model.Entity, bool) {
	ctx := c.Request.Context()
	authEntityID := auth.GetEntityID(c)
	authEntityType := auth.GetEntityType(c)

	if requestedID == 0 || requestedID == authEntityID {
		entity, err := s.Store.GetEntityByID(ctx, authEntityID)
		if err != nil || entity == nil {
			FailWithCode(c, http.StatusNotFound, ErrCodeEntityNotFound, "entity not found")
			return nil, false
		}
		return entity, true
	}

	if authEntityType != model.EntityUser {
		FailWithCode(c, http.StatusForbidden, ErrCodePermDenied, "cannot act as another entity")
		return nil, false
	}

	entity, err := s.Store.GetEntityByID(ctx, requestedID)
	if err != nil || entity == nil {
		FailWithCode(c, http.StatusNotFound, ErrCodeEntityNotFound, "entity not found")
		return nil, false
	}
	if entity.OwnerID == nil || *entity.OwnerID != authEntityID {
		FailWithCode(c, http.StatusForbidden, ErrCodePermNotOwner, "not the owner of this entity")
		return nil, false
	}
	return entity, true
}

func (s *Server) areFriends(ctx *gin.Context, entityA, entityB int64) bool {
	friendship, err := s.Store.GetFriendship(ctx.Request.Context(), entityA, entityB)
	return err == nil && friendship != nil
}

func (s *Server) canStartDirectConversation(c *gin.Context, initiator *model.Entity, target *model.Entity) bool {
	if initiator == nil || target == nil {
		return false
	}
	if initiator.ID == target.ID {
		return false
	}
	if initiator.OwnerID != nil && *initiator.OwnerID == target.ID {
		return true
	}
	if target.OwnerID != nil && *target.OwnerID == initiator.ID {
		return true
	}
	if initiator.OwnerID != nil && target.OwnerID != nil && *initiator.OwnerID == *target.OwnerID {
		return true
	}
	if s.areFriends(c, initiator.ID, target.ID) {
		return true
	}
	return (target.EntityType == model.EntityBot || target.EntityType == model.EntityService) && target.AllowNonFriendChat
}

// GET /entities/discover?q=...
func (s *Server) HandleSearchDiscoverableEntities(c *gin.Context) {
	query := strings.TrimSpace(c.Query("q"))
	limit := 20
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}
	entities, err := s.Store.SearchDiscoverableEntities(c.Request.Context(), query, limit, auth.GetEntityID(c))
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to search entities")
		return
	}
	s.attachEntitiesIdentity(c.Request.Context(), entities)
	if entities == nil {
		entities = []*model.Entity{}
	}
	OK(c, http.StatusOK, entities)
}

// GET /friends?entity_id=
func (s *Server) HandleListFriends(c *gin.Context) {
	entityID, _ := strconv.ParseInt(strings.TrimSpace(c.Query("entity_id")), 10, 64)
	entity, ok := s.resolveActingEntity(c, entityID)
	if !ok {
		return
	}
	friends, err := s.Store.ListFriends(c.Request.Context(), entity.ID)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to list friends")
		return
	}
	s.attachEntitiesIdentity(c.Request.Context(), friends)
	if friends == nil {
		friends = []*model.Entity{}
	}
	OK(c, http.StatusOK, friends)
}

// GET /friends/requests
func (s *Server) HandleListFriendRequests(c *gin.Context) {
	entityID, _ := strconv.ParseInt(strings.TrimSpace(c.Query("entity_id")), 10, 64)
	entity, ok := s.resolveActingEntity(c, entityID)
	if !ok {
		return
	}
	direction := strings.TrimSpace(c.Query("direction"))
	status := strings.TrimSpace(c.Query("status"))
	reqs, err := s.Store.ListFriendRequestsByEntity(c.Request.Context(), entity.ID, direction, status)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to list friend requests")
		return
	}
	for _, req := range reqs {
		s.attachEntityIdentity(c.Request.Context(), req.SourceEntity)
		s.attachEntityIdentity(c.Request.Context(), req.TargetEntity)
	}
	if reqs == nil {
		reqs = []*model.FriendRequest{}
	}
	OK(c, http.StatusOK, reqs)
}

// POST /friends/requests
func (s *Server) HandleCreateFriendRequest(c *gin.Context) {
	var req struct {
		SourceEntityID int64  `json:"source_entity_id"`
		TargetEntityID int64  `json:"target_entity_id" binding:"required"`
		Message        string `json:"message"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	source, ok := s.resolveActingEntity(c, req.SourceEntityID)
	if !ok {
		return
	}
	if req.TargetEntityID == 0 || req.TargetEntityID == source.ID {
		FailWithCode(c, http.StatusBadRequest, ErrCodeValidationField, "target_entity_id is invalid")
		return
	}
	target, err := s.Store.GetEntityByID(c.Request.Context(), req.TargetEntityID)
	if err != nil || target == nil {
		FailWithCode(c, http.StatusNotFound, ErrCodeEntityNotFound, "entity not found")
		return
	}
	if target.Status != "active" {
		FailWithCode(c, http.StatusBadRequest, ErrCodeStateBadTransition, "target entity is not active")
		return
	}
	if s.areFriends(c, source.ID, target.ID) {
		friendship, _ := s.Store.GetFriendship(c.Request.Context(), source.ID, target.ID)
		OK(c, http.StatusOK, gin.H{"friendship": friendship, "already_friends": true})
		return
	}
	if existing, err := s.Store.FindPendingFriendRequest(c.Request.Context(), source.ID, target.ID); err == nil && existing != nil {
		OK(c, http.StatusOK, existing)
		return
	}
	if reverse, err := s.Store.FindPendingFriendRequest(c.Request.Context(), target.ID, source.ID); err == nil && reverse != nil {
		reverse.Status = model.FriendRequestAccepted
		resolver := source.ID
		reverse.ResolvedBy = &resolver
		if err := s.Store.UpdateFriendRequest(c.Request.Context(), reverse); err != nil {
			Fail(c, http.StatusInternalServerError, "failed to accept friend request")
			return
		}
		low, high := orderedFriendEntityPair(source.ID, target.ID)
		friendship := &model.Friendship{EntityLowID: low, EntityHighID: high, CreatedBy: source.ID}
		if err := s.Store.CreateFriendship(c.Request.Context(), friendship); err != nil {
			Fail(c, http.StatusInternalServerError, "failed to create friendship")
			return
		}
		OK(c, http.StatusCreated, gin.H{"auto_accepted": true, "friendship": friendship, "request": reverse})
		return
	}

	friendReq := &model.FriendRequest{
		SourceEntityID: source.ID,
		TargetEntityID: target.ID,
		Message:        strings.TrimSpace(req.Message),
		Status:         model.FriendRequestPending,
	}
	if err := s.Store.CreateFriendRequest(c.Request.Context(), friendReq); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to create friend request")
		return
	}
	friendReq.SourceEntity = source
	friendReq.TargetEntity = target
	s.attachEntityIdentity(c.Request.Context(), source)
	s.attachEntityIdentity(c.Request.Context(), target)
	OK(c, http.StatusCreated, friendReq)
}

func (s *Server) resolveFriendRequestTarget(c *gin.Context) (*model.FriendRequest, *model.Entity, bool) {
	reqID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid friend request id")
		return nil, nil, false
	}
	friendReq, err := s.Store.GetFriendRequestByID(c.Request.Context(), reqID)
	if err != nil || friendReq == nil {
		Fail(c, http.StatusNotFound, "friend request not found")
		return nil, nil, false
	}
	actingEntityID, _ := strconv.ParseInt(strings.TrimSpace(c.Query("entity_id")), 10, 64)
	entity, ok := s.resolveActingEntity(c, actingEntityID)
	if !ok {
		return nil, nil, false
	}
	return friendReq, entity, true
}

// POST /friends/requests/:id/accept
func (s *Server) HandleAcceptFriendRequest(c *gin.Context) {
	friendReq, actor, ok := s.resolveFriendRequestTarget(c)
	if !ok {
		return
	}
	if friendReq.Status != model.FriendRequestPending {
		FailWithCode(c, http.StatusBadRequest, ErrCodeStateBadTransition, "friend request is not pending")
		return
	}
	if friendReq.TargetEntityID != actor.ID {
		FailWithCode(c, http.StatusForbidden, ErrCodePermDenied, "cannot accept this request")
		return
	}
	friendReq.Status = model.FriendRequestAccepted
	friendReq.ResolvedBy = &actor.ID
	if err := s.Store.UpdateFriendRequest(c.Request.Context(), friendReq); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to accept friend request")
		return
	}
	low, high := orderedFriendEntityPair(friendReq.SourceEntityID, friendReq.TargetEntityID)
	friendship := &model.Friendship{EntityLowID: low, EntityHighID: high, CreatedBy: actor.ID}
	if err := s.Store.CreateFriendship(c.Request.Context(), friendship); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to create friendship")
		return
	}
	OK(c, http.StatusOK, gin.H{"request": friendReq, "friendship": friendship})
}

func (s *Server) updateFriendRequestStatus(c *gin.Context, expectedEntityID int64, status model.FriendRequestStatus, message string) {
	friendReq, actor, ok := s.resolveFriendRequestTarget(c)
	if !ok {
		return
	}
	if friendReq.Status != model.FriendRequestPending {
		FailWithCode(c, http.StatusBadRequest, ErrCodeStateBadTransition, "friend request is not pending")
		return
	}
	if expectedEntityID == friendReq.TargetEntityID && friendReq.TargetEntityID != actor.ID {
		FailWithCode(c, http.StatusForbidden, ErrCodePermDenied, "cannot update this request")
		return
	}
	if expectedEntityID == friendReq.SourceEntityID && friendReq.SourceEntityID != actor.ID {
		FailWithCode(c, http.StatusForbidden, ErrCodePermDenied, "cannot update this request")
		return
	}
	friendReq.Status = status
	friendReq.ResolvedBy = &actor.ID
	if err := s.Store.UpdateFriendRequest(c.Request.Context(), friendReq); err != nil {
		Fail(c, http.StatusInternalServerError, message)
		return
	}
	OK(c, http.StatusOK, friendReq)
}

// POST /friends/requests/:id/reject
func (s *Server) HandleRejectFriendRequest(c *gin.Context) {
	friendReq, _, ok := s.resolveFriendRequestTarget(c)
	if !ok {
		return
	}
	s.updateFriendRequestStatus(c, friendReq.TargetEntityID, model.FriendRequestRejected, "failed to reject friend request")
}

// POST /friends/requests/:id/cancel
func (s *Server) HandleCancelFriendRequest(c *gin.Context) {
	friendReq, _, ok := s.resolveFriendRequestTarget(c)
	if !ok {
		return
	}
	s.updateFriendRequestStatus(c, friendReq.SourceEntityID, model.FriendRequestCanceled, "failed to cancel friend request")
}

// DELETE /friends/:entityId?entity_id=
func (s *Server) HandleDeleteFriend(c *gin.Context) {
	targetID, err := strconv.ParseInt(c.Param("entityId"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid target entity id")
		return
	}
	actingID, _ := strconv.ParseInt(strings.TrimSpace(c.Query("entity_id")), 10, 64)
	source, ok := s.resolveActingEntity(c, actingID)
	if !ok {
		return
	}
	if err := s.Store.DeleteFriendship(c.Request.Context(), source.ID, targetID); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to delete friendship")
		return
	}
	OK(c, http.StatusOK, gin.H{"entity_id": source.ID, "removed_friend_id": targetID})
}

func normalizeDiscoverability(entity *model.Entity) {
	if entity == nil {
		return
	}
	if entity.Discoverability != "" {
		return
	}
	meta := map[string]any{}
	if len(entity.Metadata) > 0 {
		_ = json.Unmarshal(entity.Metadata, &meta)
	}
	if value, ok := meta["discoverability"].(string); ok && value != "" {
		entity.Discoverability = value
		return
	}
	entity.Discoverability = "private"
}

func validateDiscoverability(value string) bool {
	switch value {
	case "", "private", "platform_public", "external_public":
		return true
	default:
		return false
	}
}
