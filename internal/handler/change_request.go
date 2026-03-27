package handler

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/wzfukui/agent-native-im/internal/auth"
	"github.com/wzfukui/agent-native-im/internal/model"
)

// POST /conversations/:id/change-requests
func (s *Server) HandleCreateChangeRequest(c *gin.Context) {
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
		Field    string `json:"field" binding:"required"`
		NewValue string `json:"new_value" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "field and new_value are required")
		return
	}

	// Get old value
	conv, err := s.Store.GetConversation(ctx, convID)
	if err != nil {
		FailWithCode(c, http.StatusNotFound, ErrCodeConvNotFound, "conversation not found")
		return
	}

	var oldValue string
	switch req.Field {
	case "title":
		oldValue = conv.Title
	case "description":
		oldValue = conv.Description
	case "prompt":
		oldValue = conv.Prompt
	default:
		Fail(c, http.StatusBadRequest, "invalid field: must be title, description, or prompt")
		return
	}

	cr := &model.ChangeRequest{
		ConversationID: convID,
		Field:          req.Field,
		OldValue:       oldValue,
		NewValue:       req.NewValue,
		RequesterID:    entityID,
		Status:         model.CRPending,
	}

	if err := s.Store.CreateChangeRequest(ctx, cr); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to create change request")
		return
	}

	// Enrich
	requester, _ := s.Store.GetEntityByID(ctx, entityID)
	cr.Requester = requester
	s.attachEntityIdentity(ctx, requester)
	convTitle := "Conversation"
	if conv != nil && conv.Title != "" {
		convTitle = conv.Title
	}
	requesterPublicID := ""
	if requester != nil {
		requesterPublicID = requester.PublicID
	}

	for _, participant := range conv.Participants {
		if participant.EntityID == entityID {
			continue
		}
		if participant.Role != model.RoleOwner && participant.Role != model.RoleAdmin {
			continue
		}
		_, _ = s.createNotificationForRecipient(
			ctx,
			participant.EntityID,
			&entityID,
			"conversation.change_request",
			"Conversation change requested",
			fmt.Sprintf("%s requested to change %s in %s", getEntityDisplayName(requester), req.Field, convTitle),
			map[string]any{
				"conversation_id":         convID,
				"conversation_title":      convTitle,
				"conversation_public_id":  conversationPublicID(conv),
				"change_request_id":       cr.ID,
				"field":                   cr.Field,
				"old_value":               cr.OldValue,
				"new_value":               cr.NewValue,
				"requester_id":            cr.RequesterID,
				"requester_public_id":     requesterPublicID,
				"requester_display_name":  getEntityDisplayName(requester),
			},
		)
	}

	// Broadcast to notify owners
	if s.Hub != nil {
		s.Hub.BroadcastEvent(convID, "conversation.change_request", map[string]interface{}{
			"change_request": cr,
		})
	}

	s.broadcastSystemMessage(c, convID, entityID,
		fmt.Sprintf("%s requested to change %s", getEntityDisplayName(requester), req.Field))

	OK(c, http.StatusCreated, cr)
}

// GET /conversations/:id/change-requests
func (s *Server) HandleListChangeRequests(c *gin.Context) {
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

	status := c.Query("status")
	crs, err := s.Store.ListChangeRequests(ctx, convID, status)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to list change requests")
		return
	}
	if crs == nil {
		crs = []*model.ChangeRequest{}
	}

	// Enrich with requester info
	for _, cr := range crs {
		cr.Requester, _ = s.Store.GetEntityByID(ctx, cr.RequesterID)
	}

	OK(c, http.StatusOK, crs)
}

// POST /conversations/:id/change-requests/:reqId/approve
func (s *Server) HandleApproveChangeRequest(c *gin.Context) {
	s.resolveChangeRequest(c, true)
}

// POST /conversations/:id/change-requests/:reqId/reject
func (s *Server) HandleRejectChangeRequest(c *gin.Context) {
	s.resolveChangeRequest(c, false)
}

func (s *Server) resolveChangeRequest(c *gin.Context, approved bool) {
	convID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid conversation id")
		return
	}

	reqID, err := strconv.ParseInt(c.Param("reqId"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid request id")
		return
	}

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	// Only owner can approve/reject
	p, err := s.Store.GetParticipant(ctx, convID, entityID)
	if err != nil || p == nil || p.Role != model.RoleOwner {
		FailWithCode(c, http.StatusForbidden, ErrCodePermNotOwner, "only owner can approve or reject change requests")
		return
	}

	cr, err := s.Store.GetChangeRequest(ctx, reqID)
	if err != nil {
		FailWithCode(c, http.StatusNotFound, ErrCodeNotFound, "change request not found")
		return
	}

	if cr.ConversationID != convID {
		Fail(c, http.StatusBadRequest, "change request does not belong to this conversation")
		return
	}

	if cr.Status != model.CRPending {
		FailWithCode(c, http.StatusConflict, ErrCodeAlreadyResolved, "change request already resolved")
		return
	}

	if err := s.Store.ResolveChangeRequest(ctx, reqID, entityID, approved); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to resolve change request")
		return
	}

	// If approved, apply the change
	if approved {
		conv, err := s.Store.GetConversation(ctx, convID)
		if err == nil {
			switch cr.Field {
			case "title":
				conv.Title = cr.NewValue
			case "description":
				conv.Description = cr.NewValue
			case "prompt":
				conv.Prompt = cr.NewValue
			}
			_ = s.Store.UpdateConversation(ctx, conv)

			// Broadcast conversation update
			if s.Hub != nil {
				s.Hub.BroadcastEvent(convID, "conversation.updated", map[string]interface{}{
					"conversation_id": convID,
					"title":           conv.Title,
					"description":     conv.Description,
					"prompt":          conv.Prompt,
					"updated_by":      entityID,
				})
			}
		}
	}

	// Broadcast resolution
	eventType := "conversation.change_rejected"
	if approved {
		eventType = "conversation.change_approved"
	}
	if s.Hub != nil {
		s.Hub.BroadcastEvent(convID, eventType, map[string]interface{}{
			"change_request_id": reqID,
			"approved":          approved,
			"approver_id":       entityID,
		})
	}

	action := "rejected"
	if approved {
		action = "approved"
	}
	approver, _ := s.Store.GetEntityByID(ctx, entityID)
	requester, _ := s.Store.GetEntityByID(ctx, cr.RequesterID)
	conv, _ := s.Store.GetConversation(ctx, convID)
	convTitle := "Conversation"
	if conv != nil && conv.Title != "" {
		convTitle = conv.Title
	}
	requesterPublicID := ""
	if requester != nil {
		requesterPublicID = requester.PublicID
	}
	approverPublicID := ""
	if approver != nil {
		approverPublicID = approver.PublicID
	}
	s.attachEntityIdentity(ctx, approver)
	s.attachEntityIdentity(ctx, requester)
	_, _ = s.createNotificationForRecipient(
		ctx,
		cr.RequesterID,
		&entityID,
		"conversation.change_"+action,
		"Conversation change request "+action,
		fmt.Sprintf("%s %s your request to change %s in %s", getEntityDisplayName(approver), action, cr.Field, convTitle),
		map[string]any{
			"conversation_id":         convID,
			"conversation_title":      convTitle,
			"conversation_public_id":  conversationPublicID(conv),
			"change_request_id":       cr.ID,
			"field":                   cr.Field,
			"old_value":               cr.OldValue,
			"new_value":               cr.NewValue,
			"status":                  map[bool]string{true: string(model.CRApproved), false: string(model.CRRejected)}[approved],
			"requester_id":            cr.RequesterID,
			"requester_public_id":     requesterPublicID,
			"approver_id":             entityID,
			"approver_public_id":      approverPublicID,
			"approver_display_name":   getEntityDisplayName(approver),
		},
	)
	s.broadcastSystemMessage(c, convID, entityID,
		fmt.Sprintf("%s %s the change request for %s", getEntityDisplayName(approver), action, cr.Field))

	OK(c, http.StatusOK, gin.H{"approved": approved})
}
