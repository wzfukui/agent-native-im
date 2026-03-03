package handler

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wzfukui/agent-native-im/internal/auth"
	"github.com/wzfukui/agent-native-im/internal/model"
)

func generateInviteCode() string {
	b := make([]byte, 12)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// POST /conversations/:id/invite
func (s *Server) HandleCreateInviteLink(c *gin.Context) {
	entityID := auth.GetEntityID(c)
	convID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid conversation id")
		return
	}

	// Check participant and role
	p, err := s.Store.GetParticipant(c, convID, entityID)
	if err != nil || p == nil {
		FailWithCode(c, http.StatusForbidden, ErrCodePermNotParticipant, "not a participant")
		return
	}
	if p.Role != model.RoleOwner && p.Role != model.RoleAdmin {
		FailWithCode(c, http.StatusForbidden, ErrCodePermNotAdmin, "only owner/admin can create invite links")
		return
	}

	var req struct {
		MaxUses   int    `json:"max_uses"`
		ExpiresIn int    `json:"expires_in"` // seconds
	}
	c.ShouldBindJSON(&req)

	link := &model.InviteLink{
		ConversationID: convID,
		Code:           generateInviteCode(),
		CreatedBy:      entityID,
		MaxUses:        req.MaxUses,
	}
	if req.ExpiresIn > 0 {
		t := time.Now().Add(time.Duration(req.ExpiresIn) * time.Second)
		link.ExpiresAt = &t
	}

	if err := s.Store.CreateInviteLink(c, link); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to create invite link")
		return
	}

	OK(c, http.StatusOK, link)
}

// GET /conversations/:id/invites
func (s *Server) HandleListInviteLinks(c *gin.Context) {
	entityID := auth.GetEntityID(c)
	convID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid conversation id")
		return
	}

	// Check participant
	ok, _ := s.Store.IsParticipant(c, convID, entityID)
	if !ok {
		FailWithCode(c, http.StatusForbidden, ErrCodePermNotParticipant, "not a participant")
		return
	}

	links, err := s.Store.ListInviteLinks(c, convID)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to list invite links")
		return
	}
	if links == nil {
		links = []*model.InviteLink{}
	}

	OK(c, http.StatusOK, links)
}

// GET /invite/:code
func (s *Server) HandleGetInviteInfo(c *gin.Context) {
	code := c.Param("code")
	link, err := s.Store.GetInviteLinkByCode(c, code)
	if err != nil {
		FailWithCode(c, http.StatusNotFound, ErrCodeInviteNotFound, "invite link not found")
		return
	}

	// Check expiry
	if link.ExpiresAt != nil && time.Now().After(*link.ExpiresAt) {
		FailWithCode(c, http.StatusGone, ErrCodeStateExpired, "invite link expired")
		return
	}
	// Check max uses
	if link.MaxUses > 0 && link.UseCount >= link.MaxUses {
		FailWithCode(c, http.StatusGone, ErrCodeStateLimitReached, "invite link max uses reached")
		return
	}

	// Get conversation info
	conv, _ := s.Store.GetConversation(c, link.ConversationID)
	OK(c, http.StatusOK, gin.H{
		"invite": link,
		"conversation": conv,
	})
}

// POST /invite/:code/join
func (s *Server) HandleJoinViaInvite(c *gin.Context) {
	entityID := auth.GetEntityID(c)
	code := c.Param("code")

	link, err := s.Store.GetInviteLinkByCode(c, code)
	if err != nil {
		FailWithCode(c, http.StatusNotFound, ErrCodeInviteNotFound, "invite link not found")
		return
	}

	// Validate
	if link.ExpiresAt != nil && time.Now().After(*link.ExpiresAt) {
		FailWithCode(c, http.StatusGone, ErrCodeStateExpired, "invite link expired")
		return
	}
	if link.MaxUses > 0 && link.UseCount >= link.MaxUses {
		FailWithCode(c, http.StatusGone, ErrCodeStateLimitReached, "invite link max uses reached")
		return
	}

	// Check if already participant
	already, _ := s.Store.IsParticipant(c, link.ConversationID, entityID)
	if already {
		FailWithCode(c, http.StatusConflict, ErrCodeAlreadyMember, "already a participant")
		return
	}

	// Add as member
	p := &model.Participant{
		ConversationID:   link.ConversationID,
		EntityID:         entityID,
		Role:             model.RoleMember,
		SubscriptionMode: model.SubSubscribeAll,
	}
	if err := s.Store.AddParticipant(c, p); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to join")
		return
	}

	// Increment use count
	s.Store.IncrementInviteUseCount(c, code)

	// Subscribe to WS
	if s.Hub != nil {
		s.Hub.SubscribeEntity(link.ConversationID, entityID)
	}

	// System message
	ent, _ := s.Store.GetEntityByID(c, entityID)
	name := "Unknown"
	if ent != nil {
		name = getEntityDisplayName(ent)
	}
	s.broadcastSystemMessage(c, link.ConversationID, entityID, name+" joined via invite link")

	// Broadcast update
	if s.Hub != nil {
		s.Hub.BroadcastEvent(link.ConversationID, "conversation.updated", map[string]interface{}{
			"conversation_id": link.ConversationID,
			"action":          "member_joined",
			"entity_id":       entityID,
		})
	}

	conv, _ := s.Store.GetConversation(c, link.ConversationID)
	OK(c, http.StatusOK, conv)
}

// DELETE /invites/:id
func (s *Server) HandleDeleteInviteLink(c *gin.Context) {
	entityID := auth.GetEntityID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	// Look up the invite link to verify authorization
	link, err := s.Store.GetInviteLinkByID(c, id)
	if err != nil {
		FailWithCode(c, http.StatusNotFound, ErrCodeInviteNotFound, "invite link not found")
		return
	}

	// Only the creator or conversation owner/admin can delete
	if link.CreatedBy != entityID {
		p, err := s.Store.GetParticipant(c, link.ConversationID, entityID)
		if err != nil || p == nil || (p.Role != model.RoleOwner && p.Role != model.RoleAdmin) {
			FailWithCode(c, http.StatusForbidden, ErrCodePermDenied, "only creator or admin can delete invite links")
			return
		}
	}

	if err := s.Store.DeleteInviteLink(c, id); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to delete")
		return
	}
	OK(c, http.StatusOK, nil)
}
