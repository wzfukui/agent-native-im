package handler

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wzfukui/agent-native-im/internal/auth"
	"github.com/wzfukui/agent-native-im/internal/model"
)

// getEntityDisplayName returns display_name or name as fallback.
func getEntityDisplayName(e *model.Entity) string {
	if e == nil {
		return "unknown"
	}
	if e.DisplayName != "" {
		return e.DisplayName
	}
	return e.Name
}

// broadcastSystemMessage persists a system message and broadcasts it.
func (s *Server) broadcastSystemMessage(c *gin.Context, convID, senderID int64, summary string) {
	ctx := c.Request.Context()
	sysMsg := &model.Message{
		ConversationID: convID,
		SenderID:       senderID,
		ContentType:    model.ContentSystem,
		Layers:         model.MessageLayers{Summary: summary},
	}
	if err := s.Store.CreateMessage(ctx, sysMsg); err != nil {
		return
	}
	sender, err := s.Store.GetEntityByID(ctx, senderID)
	if err == nil && sender != nil {
		sysMsg.SenderType = string(sender.EntityType)
		sysMsg.Sender = sender
	}
	if s.Hub != nil {
		s.Hub.BroadcastMessage(sysMsg)
	}
}

type createConversationRequest struct {
	Title          string  `json:"title"`
	Description    string  `json:"description"`
	ConvType       string  `json:"conv_type"`
	ParticipantIDs []int64 `json:"participant_ids"`
}

func generateConversationID() int64 {
	var b [8]byte
	maxSafeInt := uint64(math.MaxInt64)
	if (1 << 53) < maxSafeInt {
		maxSafeInt = (1 << 53) - 1
	}
	for i := 0; i < 5; i++ {
		if _, err := rand.Read(b[:]); err != nil {
			break
		}
		// Positive non-zero random ID constrained to JS safe integer range.
		id := binary.BigEndian.Uint64(b[:]) & maxSafeInt
		if id != 0 {
			return int64(id)
		}
	}
	// Fallback for rare entropy source errors.
	if fallback := time.Now().UnixNano() & int64(maxSafeInt); fallback > 0 {
		return fallback
	}
	return 1
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
		ID:          generateConversationID(),
		ConvType:    convType,
		Title:       req.Title,
		Description: req.Description,
	}
	meta, _ := json.Marshal(map[string]interface{}{"public_id": uuid.NewString()})
	conv.Metadata = meta

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
	ctx := c.Request.Context()

	var convs []*model.Conversation
	var err error
	if c.Query("archived") == "true" {
		convs, err = s.Store.ListArchivedConversationsByEntity(ctx, entityID)
	} else {
		convs, err = s.Store.ListConversationsByEntity(ctx, entityID)
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to list conversations")
		return
	}

	// Enrich with unread counts
	counts, _ := s.Store.GetUnreadCounts(ctx, entityID)

	type convWithUnread struct {
		*model.Conversation
		UnreadCount int `json:"unread_count"`
	}
	result := make([]convWithUnread, len(convs))
	for i, conv := range convs {
		result[i] = convWithUnread{Conversation: conv, UnreadCount: counts[conv.ID]}
	}

	OK(c, http.StatusOK, result)
}

// HandleMarkAsRead marks messages up to a given ID as read for the caller.
func (s *Server) HandleMarkAsRead(c *gin.Context) {
	convID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid conversation id")
		return
	}

	var req struct {
		MessageID int64 `json:"message_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "message_id is required")
		return
	}

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	ok, err := s.Store.IsParticipant(ctx, convID, entityID)
	if err != nil || !ok {
		FailWithCode(c, http.StatusForbidden, ErrCodePermNotParticipant, "not a participant")
		return
	}

	if err := s.Store.MarkAsRead(ctx, convID, entityID, req.MessageID); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to mark as read")
		return
	}

	// Broadcast read receipt to other participants (exclude the reader)
	if s.Hub != nil {
		s.Hub.BroadcastEventExcluding(convID, "message.read", map[string]interface{}{
			"conversation_id": convID,
			"entity_id":       entityID,
			"message_id":      req.MessageID,
			"last_read_at":    time.Now().UTC().Format(time.RFC3339),
		}, entityID)
	}

	OK(c, http.StatusOK, gin.H{"conversation_id": convID, "last_read_message_id": req.MessageID})
}

func (s *Server) HandleGetConversation(c *gin.Context) {
	convID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid conversation id")
		return
	}

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	ok, err := s.Store.IsParticipant(ctx, convID, entityID)
	if err != nil || !ok {
		FailWithCode(c, http.StatusForbidden, ErrCodePermNotParticipant, "not a participant of this conversation")
		return
	}

	conv, err := s.Store.GetConversation(ctx, convID)
	if err != nil {
		FailWithCode(c, http.StatusNotFound, ErrCodeConvNotFound, "conversation not found")
		return
	}

	OK(c, http.StatusOK, conv)
}

func (s *Server) HandleGetConversationByPublicID(c *gin.Context) {
	publicID := strings.TrimSpace(c.Param("publicId"))
	if publicID == "" {
		Fail(c, http.StatusBadRequest, "invalid public conversation id")
		return
	}

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	conv, err := s.Store.GetConversationByPublicID(ctx, publicID)
	if err != nil {
		FailWithCode(c, http.StatusNotFound, ErrCodeConvNotFound, "conversation not found")
		return
	}

	ok, err := s.Store.IsParticipant(ctx, conv.ID, entityID)
	if err != nil || !ok {
		FailWithCode(c, http.StatusForbidden, ErrCodePermNotParticipant, "not a participant of this conversation")
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

	// Only owner/admin can add participants
	caller, err := s.Store.GetParticipant(ctx, convID, entityID)
	if err != nil || caller == nil {
		FailWithCode(c, http.StatusForbidden, ErrCodePermNotParticipant, "not a participant of this conversation")
		return
	}

	target, err := s.Store.GetEntityByID(ctx, req.EntityID)
	if err != nil || target == nil {
		Fail(c, http.StatusNotFound, "entity not found")
		return
	}

	canManageMembers := caller.Role == model.RoleOwner || caller.Role == model.RoleAdmin
	if !canManageMembers {
		conv, err := s.Store.GetConversation(ctx, convID)
		if err != nil || conv == nil {
			Fail(c, http.StatusNotFound, "conversation not found")
			return
		}
		if conv.ConvType != model.ConvGroup && conv.ConvType != model.ConvChannel {
			FailWithCode(c, http.StatusForbidden, ErrCodePermNotAdmin, "members can only add their own bots to group or channel conversations")
			return
		}
		// Regular members can only add their own bots as regular members.
		if target.EntityType != model.EntityBot || target.OwnerID == nil || *target.OwnerID != entityID {
			FailWithCode(c, http.StatusForbidden, ErrCodePermNotAdmin, "only owner or admin can add other participants")
			return
		}
		if req.Role != "" && req.Role != "member" {
			FailWithCode(c, http.StatusForbidden, ErrCodePermNotAdmin, "members can only add their own bots as members")
			return
		}
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

	// Subscribe entity to WS and broadcast
	if s.Hub != nil {
		s.Hub.SubscribeEntity(convID, req.EntityID)
		s.Hub.NotifyNewConversation(convID, req.EntityID)
	}

	// System message
	adder, _ := s.Store.GetEntityByID(ctx, entityID)
	s.broadcastSystemMessage(c, convID, entityID,
		fmt.Sprintf("%s 邀请 %s 加入了群聊", getEntityDisplayName(adder), getEntityDisplayName(target)))

	// Broadcast conversation update
	if s.Hub != nil {
		s.Hub.BroadcastEvent(convID, "conversation.updated", map[string]interface{}{
			"conversation_id": convID,
			"action":          "member_added",
			"entity_id":       req.EntityID,
		})
	}

	OK(c, http.StatusCreated, "participant added")
}

// HandleUpdateSubscription updates the caller's subscription mode for a conversation.
func (s *Server) HandleUpdateSubscription(c *gin.Context) {
	convID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid conversation id")
		return
	}

	var req struct {
		Mode          string `json:"mode" binding:"required"`
		ContextWindow *int   `json:"context_window"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "mode is required")
		return
	}

	validModes := map[model.SubscriptionMode]bool{
		model.SubMentionOnly:     true,
		model.SubSubscribeAll:    true,
		model.SubMentionWithCtx:  true,
		model.SubSubscribeDigest: true,
	}
	mode := model.SubscriptionMode(req.Mode)
	if !validModes[mode] {
		Fail(c, http.StatusBadRequest, "mode must be mention_only, subscribe_all, mention_with_context, or subscribe_digest")
		return
	}

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	ok, err := s.Store.IsParticipant(ctx, convID, entityID)
	if err != nil || !ok {
		FailWithCode(c, http.StatusForbidden, ErrCodePermNotParticipant, "not a participant of this conversation")
		return
	}

	contextWindow := 5
	if req.ContextWindow != nil && *req.ContextWindow > 0 {
		contextWindow = *req.ContextWindow
		if contextWindow > 50 {
			contextWindow = 50
		}
	}
	if err := s.Store.UpdateSubscriptionWithContext(ctx, convID, entityID, mode, contextWindow); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to update subscription")
		return
	}

	OK(c, http.StatusOK, gin.H{"mode": mode, "context_window": contextWindow})
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

	caller, err := s.Store.GetParticipant(ctx, convID, entityID)
	if err != nil || caller == nil {
		FailWithCode(c, http.StatusForbidden, ErrCodePermNotParticipant, "not a participant of this conversation")
		return
	}

	if targetID != entityID {
		if caller.Role != model.RoleOwner && caller.Role != model.RoleAdmin {
			FailWithCode(c, http.StatusForbidden, ErrCodePermNotAdmin, "only owner or admin can remove other participants")
			return
		}
	}

	if err := s.Store.RemoveParticipant(ctx, convID, targetID); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to remove participant")
		return
	}

	// System message
	remover, _ := s.Store.GetEntityByID(ctx, entityID)
	removed, _ := s.Store.GetEntityByID(ctx, targetID)
	var summary string
	if targetID == entityID {
		summary = fmt.Sprintf("%s 离开了群聊", getEntityDisplayName(remover))
	} else {
		summary = fmt.Sprintf("%s 移除了 %s", getEntityDisplayName(remover), getEntityDisplayName(removed))
	}
	s.broadcastSystemMessage(c, convID, entityID, summary)

	// Unsubscribe from WS and broadcast
	if s.Hub != nil {
		s.Hub.UnsubscribeEntity(convID, targetID)
		s.Hub.BroadcastEvent(convID, "conversation.updated", map[string]interface{}{
			"conversation_id": convID,
			"action":          "member_removed",
			"entity_id":       targetID,
		})
	}

	OK(c, http.StatusOK, "participant removed")
}

// HandleUpdateConversation updates a conversation's title and/or description.
func (s *Server) HandleUpdateConversation(c *gin.Context) {
	convID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid conversation id")
		return
	}

	var req struct {
		Title       *string `json:"title"`
		Description *string `json:"description"`
		Prompt      *string `json:"prompt"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	if req.Title == nil && req.Description == nil && req.Prompt == nil {
		Fail(c, http.StatusBadRequest, "nothing to update")
		return
	}

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	ok, err := s.Store.IsParticipant(ctx, convID, entityID)
	if err != nil || !ok {
		FailWithCode(c, http.StatusForbidden, ErrCodePermNotParticipant, "not a participant of this conversation")
		return
	}

	conv, err := s.Store.GetConversation(ctx, convID)
	if err != nil {
		FailWithCode(c, http.StatusNotFound, ErrCodeConvNotFound, "conversation not found")
		return
	}

	// Permission check for groups/channels
	if conv.ConvType == model.ConvGroup || conv.ConvType == model.ConvChannel {
		participant, err := s.Store.GetParticipant(ctx, convID, entityID)
		if err != nil || (participant.Role != model.RoleOwner && participant.Role != model.RoleAdmin) {
			FailWithCode(c, http.StatusForbidden, ErrCodePermNotAdmin, "only owner or admin can update this conversation")
			return
		}
	}

	// Build system message summary
	sender, _ := s.Store.GetEntityByID(ctx, entityID)
	senderName := getEntityDisplayName(sender)
	var summaryParts []string

	if req.Title != nil {
		conv.Title = *req.Title
		summaryParts = append(summaryParts, fmt.Sprintf("%s 修改了群名为 \"%s\"", senderName, *req.Title))
	}
	if req.Description != nil {
		conv.Description = *req.Description
		summaryParts = append(summaryParts, fmt.Sprintf("%s 修改了群描述", senderName))
	}
	if req.Prompt != nil {
		conv.Prompt = *req.Prompt
		summaryParts = append(summaryParts, fmt.Sprintf("%s 更新了会话提示词", senderName))
	}

	if err := s.Store.UpdateConversation(ctx, conv); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to update conversation")
		return
	}

	// System message and broadcast
	if len(summaryParts) > 0 {
		s.broadcastSystemMessage(c, convID, entityID, strings.Join(summaryParts, "；"))
	}

	if s.Hub != nil {
		s.Hub.BroadcastEvent(convID, "conversation.updated", map[string]interface{}{
			"conversation_id": convID,
			"title":           conv.Title,
			"description":     conv.Description,
			"prompt":          conv.Prompt,
			"updated_by":      entityID,
		})
	}

	OK(c, http.StatusOK, conv)
}

// HandleLeaveConversation allows a participant to leave a conversation.
func (s *Server) HandleLeaveConversation(c *gin.Context) {
	convID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid conversation id")
		return
	}

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	participant, err := s.Store.GetParticipant(ctx, convID, entityID)
	if err != nil || participant == nil {
		FailWithCode(c, http.StatusForbidden, ErrCodePermNotParticipant, "not a participant of this conversation")
		return
	}

	// If owner is leaving, transfer ownership to next admin or oldest member
	if participant.Role == model.RoleOwner {
		participants, _ := s.Store.ListParticipants(ctx, convID)
		var newOwner *model.Participant
		for _, p := range participants {
			if p.EntityID == entityID {
				continue
			}
			if p.Role == model.RoleAdmin {
				newOwner = p
				break
			}
			if newOwner == nil {
				newOwner = p
			}
		}
		if newOwner != nil {
			_ = s.Store.UpdateParticipantRole(ctx, convID, newOwner.EntityID, model.RoleOwner)
			newOwnerEntity, _ := s.Store.GetEntityByID(ctx, newOwner.EntityID)
			s.broadcastSystemMessage(c, convID, entityID,
				fmt.Sprintf("群主已转让给 %s", getEntityDisplayName(newOwnerEntity)))
		}
	}

	if err := s.Store.RemoveParticipant(ctx, convID, entityID); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to leave conversation")
		return
	}

	leaver, _ := s.Store.GetEntityByID(ctx, entityID)
	s.broadcastSystemMessage(c, convID, entityID,
		fmt.Sprintf("%s 离开了群聊", getEntityDisplayName(leaver)))

	if s.Hub != nil {
		s.Hub.UnsubscribeEntity(convID, entityID)
		s.Hub.BroadcastEvent(convID, "conversation.updated", map[string]interface{}{
			"conversation_id": convID,
			"action":          "member_left",
			"entity_id":       entityID,
		})
	}

	OK(c, http.StatusOK, "left conversation")
}

// HandleArchiveConversation archives a conversation for the caller.
func (s *Server) HandleArchiveConversation(c *gin.Context) {
	convID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid conversation id")
		return
	}

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	ok, err := s.Store.IsParticipant(ctx, convID, entityID)
	if err != nil || !ok {
		FailWithCode(c, http.StatusForbidden, ErrCodePermNotParticipant, "not a participant of this conversation")
		return
	}

	if err := s.Store.ArchiveConversation(ctx, convID, entityID); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to archive conversation")
		return
	}

	OK(c, http.StatusOK, "conversation archived")
}

// HandleUnarchiveConversation unarchives a conversation for the caller.
func (s *Server) HandleUnarchiveConversation(c *gin.Context) {
	convID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid conversation id")
		return
	}

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	// Use raw check since archived convs are excluded from normal participant check
	p, err := s.Store.GetParticipant(ctx, convID, entityID)
	if err != nil || p == nil {
		FailWithCode(c, http.StatusForbidden, ErrCodePermNotParticipant, "not a participant of this conversation")
		return
	}

	if err := s.Store.UnarchiveConversation(ctx, convID, entityID); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to unarchive conversation")
		return
	}

	OK(c, http.StatusOK, "conversation unarchived")
}

// HandlePinConversation pins a conversation for the caller.
func (s *Server) HandlePinConversation(c *gin.Context) {
	convID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid conversation id")
		return
	}

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	ok, err := s.Store.IsParticipant(ctx, convID, entityID)
	if err != nil || !ok {
		FailWithCode(c, http.StatusForbidden, ErrCodePermNotParticipant, "not a participant of this conversation")
		return
	}

	if err := s.Store.PinConversation(ctx, convID, entityID); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to pin conversation")
		return
	}

	OK(c, http.StatusOK, "conversation pinned")
}

// HandleUnpinConversation unpins a conversation for the caller.
func (s *Server) HandleUnpinConversation(c *gin.Context) {
	convID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid conversation id")
		return
	}

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	ok, err := s.Store.IsParticipant(ctx, convID, entityID)
	if err != nil || !ok {
		FailWithCode(c, http.StatusForbidden, ErrCodePermNotParticipant, "not a participant of this conversation")
		return
	}

	if err := s.Store.UnpinConversation(ctx, convID, entityID); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to unpin conversation")
		return
	}

	OK(c, http.StatusOK, "conversation unpinned")
}
