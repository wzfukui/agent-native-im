package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/wzfukui/agent-native-im/internal/auth"
	"github.com/wzfukui/agent-native-im/internal/model"
)

// HandleToggleReaction adds or removes a reaction (toggle behavior).
// POST /messages/:id/reactions  body: { "emoji": "👍" }
func (s *Server) HandleToggleReaction(c *gin.Context) {
	msgID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid message id")
		return
	}

	var req struct {
		Emoji string `json:"emoji" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Emoji == "" {
		FailWithCode(c, http.StatusBadRequest, ErrCodeValidation, "emoji is required")
		return
	}

	// Limit emoji length
	if len(req.Emoji) > 32 {
		FailWithCode(c, http.StatusBadRequest, ErrCodeValidation, "emoji too long")
		return
	}

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	// Verify message exists
	msg, err := s.Store.GetMessageByID(ctx, msgID)
	if err != nil {
		FailWithCode(c, http.StatusNotFound, ErrCodeMessageNotFound, "message not found")
		return
	}

	// Verify sender is participant
	ok, err := s.Store.IsParticipant(ctx, msg.ConversationID, entityID)
	if err != nil || !ok {
		FailWithCode(c, http.StatusForbidden, ErrCodePermNotParticipant, "not a participant")
		return
	}

	// Toggle: try to add; if already exists (conflict DO NOTHING), remove instead
	reaction := &model.Reaction{
		MessageID: msgID,
		EntityID:  entityID,
		Emoji:     req.Emoji,
	}

	err = s.Store.AddReaction(ctx, reaction)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to add reaction")
		return
	}

	// Check if it was actually inserted (id > 0) or ignored by ON CONFLICT
	added := reaction.ID > 0
	if !added {
		// Already existed — remove it (toggle off)
		if err := s.Store.RemoveReaction(ctx, msgID, entityID, req.Emoji); err != nil {
			Fail(c, http.StatusInternalServerError, "failed to remove reaction")
			return
		}
	}

	// Fetch updated reactions for the message
	reactionsMap, err := s.Store.GetReactionsByMessages(ctx, []int64{msgID})
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to fetch reactions")
		return
	}

	reactions := reactionsMap[msgID]
	if reactions == nil {
		reactions = []model.ReactionSummary{}
	}

	// Broadcast event
	if s.Hub != nil {
		eventType := "message.reaction_updated"
		s.Hub.BroadcastEvent(msg.ConversationID, eventType, gin.H{
			"message_id":      msgID,
			"conversation_id": msg.ConversationID,
			"reactions":       reactions,
		})
	}

	OK(c, http.StatusOK, gin.H{
		"message_id": msgID,
		"reactions":  reactions,
	})
}
