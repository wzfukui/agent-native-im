package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"github.com/wzfukui/agent-native-im/internal/auth"
	"github.com/wzfukui/agent-native-im/internal/model"
)

const publicVisitorTokenTTL = 12 * time.Hour

func randomCode(prefix string, n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

func (s *Server) resolveOwnedBot(c *gin.Context, entityID int64) (*model.Entity, bool) {
	if auth.GetEntityType(c) != model.EntityUser {
		FailWithCode(c, http.StatusForbidden, ErrCodePermDenied, "only users can manage bot access links")
		return nil, false
	}
	target, err := s.Store.GetEntityByID(c.Request.Context(), entityID)
	if err != nil || target == nil {
		FailWithCode(c, http.StatusNotFound, ErrCodeEntityNotFound, "entity not found")
		return nil, false
	}
	if target.EntityType != model.EntityBot && target.EntityType != model.EntityService {
		FailWithCode(c, http.StatusBadRequest, ErrCodeValidationField, "entity is not a bot")
		return nil, false
	}
	if target.OwnerID == nil || *target.OwnerID != auth.GetEntityID(c) {
		FailWithCode(c, http.StatusForbidden, ErrCodePermNotOwner, "not the owner of this entity")
		return nil, false
	}
	return target, true
}

func (s *Server) resolvePublicBotCtx(c *gin.Context, identifier string) (*model.Entity, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return nil, nil
	}
	if strings.HasPrefix(identifier, "bot_") {
		return s.Store.GetEntityByBotID(c.Request.Context(), identifier)
	}
	return s.Store.GetEntityByPublicID(c.Request.Context(), identifier)
}

func (s *Server) canUsePublicBot(ctx context.Context, bot *model.Entity, accessCode string) (*model.BotAccessLink, bool) {
	if bot == nil || bot.Status != "active" || (bot.EntityType != model.EntityBot && bot.EntityType != model.EntityService) {
		return nil, false
	}
	if strings.TrimSpace(accessCode) != "" {
		link, err := s.Store.GetBotAccessLinkByCode(ctx, strings.TrimSpace(accessCode))
		if err == nil && link != nil && link.BotEntityID == bot.ID {
			if link.ExpiresAt != nil && link.ExpiresAt.Before(time.Now()) {
				return nil, false
			}
			if link.MaxUses > 0 && link.UsedCount >= link.MaxUses {
				return nil, false
			}
			return link, true
		}
	}
	return nil, bot.Discoverability == "external_public"
}

// POST /bots/:id/access-links
func (s *Server) HandleCreateBotAccessLink(c *gin.Context) {
	entityID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid bot id")
		return
	}
	bot, ok := s.resolveOwnedBot(c, entityID)
	if !ok {
		return
	}

	var req struct {
		Label     string     `json:"label"`
		ExpiresAt *time.Time `json:"expires_at"`
		MaxUses   *int       `json:"max_uses"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	maxUses := 0
	if req.MaxUses != nil {
		maxUses = *req.MaxUses
		if maxUses < 0 {
			FailWithCode(c, http.StatusBadRequest, ErrCodeValidationField, "max_uses must be >= 0")
			return
		}
	}

	link := &model.BotAccessLink{
		BotEntityID:       bot.ID,
		Code:              randomCode("bap_", 10),
		Label:             strings.TrimSpace(req.Label),
		ExpiresAt:         req.ExpiresAt,
		MaxUses:           maxUses,
		CreatedByEntityID: auth.GetEntityID(c),
	}
	if err := s.Store.CreateBotAccessLink(c.Request.Context(), link); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to create bot access link")
		return
	}
	OK(c, http.StatusCreated, link)
}

// GET /bots/:id/access-links
func (s *Server) HandleListBotAccessLinks(c *gin.Context) {
	entityID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid bot id")
		return
	}
	if _, ok := s.resolveOwnedBot(c, entityID); !ok {
		return
	}
	links, err := s.Store.ListBotAccessLinks(c.Request.Context(), entityID)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to list bot access links")
		return
	}
	if links == nil {
		links = []*model.BotAccessLink{}
	}
	OK(c, http.StatusOK, links)
}

// DELETE /bot-access-links/:id
func (s *Server) HandleDeleteBotAccessLink(c *gin.Context) {
	linkID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid access link id")
		return
	}
	link, err := s.Store.GetBotAccessLinkByID(c.Request.Context(), linkID)
	if err != nil || link == nil {
		FailWithCode(c, http.StatusNotFound, ErrCodeEntityNotFound, "access link not found")
		return
	}
	if _, ok := s.resolveOwnedBot(c, link.BotEntityID); !ok {
		return
	}
	if err := s.Store.DeleteBotAccessLink(c.Request.Context(), linkID); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to delete bot access link")
		return
	}
	OK(c, http.StatusOK, gin.H{"id": linkID})
}

func (s *Server) createPublicVisitorSession(c *gin.Context, bot *model.Entity, accessCode, displayName string) (*model.Entity, *model.Conversation, string, error) {
	ctx := c.Request.Context()
	visitorName := "visitor_" + strings.ToLower(strings.ReplaceAll(uuid.NewString(), "-", ""))[:16]
	if displayName == "" {
		displayName = "Guest"
	}
	meta, _ := json.Marshal(map[string]any{
		"public_visitor": true,
		"bot_entity_id":  bot.ID,
		"access_code":    strings.TrimSpace(accessCode),
	})
	visitor := &model.Entity{
		PublicID:       uuid.NewString(),
		EntityType:     model.EntityService,
		Name:           visitorName,
		DisplayName:    displayName,
		Status:         "active",
		Discoverability: "private",
		Metadata:       meta,
	}
	if err := s.Store.CreateEntity(ctx, visitor); err != nil {
		return nil, nil, "", err
	}

	conv := &model.Conversation{
		ID:       generateConversationID(),
		ConvType: model.ConvDirect,
		Title:    bot.DisplayName,
	}
	convMeta, _ := json.Marshal(map[string]any{"public_id": uuid.NewString(), "public_session": true})
	conv.Metadata = convMeta
	if err := s.Store.CreateConversation(ctx, conv); err != nil {
		return nil, nil, "", err
	}
	if err := s.Store.AddParticipant(ctx, &model.Participant{ConversationID: conv.ID, EntityID: visitor.ID, Role: model.RoleOwner}); err != nil {
		return nil, nil, "", err
	}
	if err := s.Store.AddParticipant(ctx, &model.Participant{ConversationID: conv.ID, EntityID: bot.ID, Role: model.RoleMember}); err != nil {
		return nil, nil, "", err
	}
	if s.Hub != nil {
		s.Hub.NotifyNewConversation(conv.ID, visitor.ID, bot.ID)
	}
	token, err := auth.GenerateTokenWithTTL(s.Config.JWTSecret, visitor.ID, visitor.EntityType, publicVisitorTokenTTL)
	if err != nil {
		return nil, nil, "", err
	}
	fullConv, err := s.Store.GetConversation(ctx, conv.ID)
	if err != nil {
		return nil, nil, "", err
	}
	return visitor, fullConv, token, nil
}

// GET /public/bots/:identifier
func (s *Server) HandleGetPublicBot(c *gin.Context) {
	bot, err := s.resolvePublicBotCtx(c, c.Param("identifier"))
	if err != nil || bot == nil {
		FailWithCode(c, http.StatusNotFound, ErrCodeEntityNotFound, "bot not found")
		return
	}
	normalizeDiscoverability(bot)
	accessCode := strings.TrimSpace(c.Query("code"))
	_, allowed := s.canUsePublicBot(c.Request.Context(), bot, accessCode)
	if !allowed {
		FailWithCode(c, http.StatusForbidden, ErrCodePermDenied, "bot is not publicly accessible")
		return
	}
	s.attachEntityIdentity(c.Request.Context(), bot)
	OK(c, http.StatusOK, gin.H{
		"bot": gin.H{
			"id":                      bot.ID,
			"public_id":               bot.PublicID,
			"bot_id":                  bot.BotID,
			"display_name":            bot.DisplayName,
			"name":                    bot.Name,
			"avatar_url":              bot.AvatarURL,
			"discoverability":         bot.Discoverability,
			"require_access_password": bot.RequireAccessPassword,
		},
		"access_code": accessCode,
	})
}

// POST /public/bots/:identifier/session
func (s *Server) HandleCreatePublicBotSession(c *gin.Context) {
	bot, err := s.resolvePublicBotCtx(c, c.Param("identifier"))
	if err != nil || bot == nil {
		FailWithCode(c, http.StatusNotFound, ErrCodeEntityNotFound, "bot not found")
		return
	}
	normalizeDiscoverability(bot)

	var req struct {
		AccessCode  string `json:"access_code"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	link, allowed := s.canUsePublicBot(c.Request.Context(), bot, req.AccessCode)
	if !allowed {
		FailWithCode(c, http.StatusForbidden, ErrCodePermDenied, "bot is not publicly accessible")
		return
	}
	if bot.RequireAccessPassword {
		if bcrypt.CompareHashAndPassword([]byte(bot.AccessPasswordHash), []byte(req.Password)) != nil {
			FailWithCode(c, http.StatusUnauthorized, ErrCodePermDenied, "invalid access password")
			return
		}
	}
	visitor, conv, token, err := s.createPublicVisitorSession(c, bot, req.AccessCode, strings.TrimSpace(req.DisplayName))
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to create public session")
		return
	}
	if link != nil {
		_ = s.Store.IncrementBotAccessLinkUseCount(c.Request.Context(), link.ID)
		link.UsedCount++
	}
	OK(c, http.StatusCreated, gin.H{
		"token":        token,
		"visitor":      visitor,
		"conversation": conv,
	})
}
