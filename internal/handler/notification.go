package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/wzfukui/agent-native-im/internal/model"
	"github.com/wzfukui/agent-native-im/internal/push"
	"github.com/wzfukui/agent-native-im/internal/ws"
)

func conversationPublicID(conv *model.Conversation) string {
	if conv == nil || len(conv.Metadata) == 0 {
		return ""
	}
	var metadata map[string]any
	if err := json.Unmarshal(conv.Metadata, &metadata); err != nil {
		return ""
	}
	if raw, ok := metadata["public_id"].(string); ok {
		return raw
	}
	return ""
}

func (s *Server) attachNotificationIdentity(ctx context.Context, notification *model.Notification) {
	if notification == nil {
		return
	}
	s.attachEntityIdentity(ctx, notification.RecipientEntity)
	s.attachEntityIdentity(ctx, notification.ActorEntity)
}

func notificationConversationID(notification *model.Notification) int64 {
	if notification == nil || len(notification.Data) == 0 {
		return 0
	}
	var payload map[string]any
	if err := json.Unmarshal(notification.Data, &payload); err != nil {
		return 0
	}
	switch raw := payload["conversation_id"].(type) {
	case float64:
		return int64(raw)
	case int64:
		return raw
	case int:
		return int64(raw)
	case string:
		id, _ := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		return id
	default:
		return 0
	}
}

func notificationConversationPublicID(notification *model.Notification) string {
	if notification == nil || len(notification.Data) == 0 {
		return ""
	}
	var payload map[string]any
	if err := json.Unmarshal(notification.Data, &payload); err != nil {
		return ""
	}
	if raw, ok := payload["conversation_public_id"].(string); ok {
		return strings.TrimSpace(raw)
	}
	return ""
}

func notificationPushPath(notification *model.Notification) string {
	if notification == nil {
		return "/inbox"
	}
	switch notification.Kind {
	case "friend.request.received", "friend.request.accepted", "friend.request.rejected", "friend.request.canceled":
		return "/friends"
	}
	if publicID := notificationConversationPublicID(notification); publicID != "" {
		return "/chat/public/" + url.PathEscape(publicID)
	}
	if convID := notificationConversationID(notification); convID > 0 {
		return "/chat/" + strconv.FormatInt(convID, 10)
	}
	return "/inbox?scope=" + url.QueryEscape(strconv.FormatInt(notification.RecipientEntityID, 10))
}

func (s *Server) pushNotification(ctx context.Context, notification *model.Notification) {
	if s.Push == nil || notification == nil {
		return
	}
	recipient := notification.RecipientEntity
	if recipient == nil {
		recipient, _ = s.Store.GetEntityByID(ctx, notification.RecipientEntityID)
	}
	if recipient == nil {
		return
	}
	targetEntityID := recipient.ID
	if recipient.EntityType != model.EntityUser {
		if recipient.OwnerID == nil || *recipient.OwnerID == 0 {
			return
		}
		targetEntityID = *recipient.OwnerID
	}
	title := strings.TrimSpace(notification.Title)
	if title == "" {
		title = "Agent-Native IM"
	}
	body := strings.TrimSpace(notification.Body)
	if body == "" {
		body = title
	}
	s.Push.SendToEntity(ctx, targetEntityID, push.Payload{
		Title: title,
		Body:  body,
		Kind:  notification.Kind,
		Path:  notificationPushPath(notification),
	})
}

func (s *Server) createNotification(c *gin.Context, recipientID int64, actorID *int64, kind, title, body string, payload map[string]any) (*model.Notification, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	notification := &model.Notification{
		RecipientEntityID: recipientID,
		ActorEntityID:     actorID,
		Kind:              kind,
		Status:            model.NotificationUnread,
		Title:             title,
		Body:              body,
		Data:              data,
	}
	if err := s.Store.CreateNotification(c.Request.Context(), notification); err != nil {
		return nil, err
	}
	notification.RecipientEntity, _ = s.Store.GetEntityByID(c.Request.Context(), recipientID)
	if actorID != nil {
		notification.ActorEntity, _ = s.Store.GetEntityByID(c.Request.Context(), *actorID)
	}
	s.attachNotificationIdentity(c.Request.Context(), notification)
	s.Hub.SendToEntity(recipientID, ws.WSMessage{
		Type: "notification.new",
		Data: notification,
	})
	s.pushNotification(c.Request.Context(), notification)
	return notification, nil
}

func (s *Server) createNotificationForRecipient(ctx context.Context, recipientID int64, actorID *int64, kind, title, body string, payload map[string]any) (*model.Notification, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	notification := &model.Notification{
		RecipientEntityID: recipientID,
		ActorEntityID:     actorID,
		Kind:              kind,
		Status:            model.NotificationUnread,
		Title:             title,
		Body:              body,
		Data:              data,
	}
	if err := s.Store.CreateNotification(ctx, notification); err != nil {
		return nil, err
	}
	notification.RecipientEntity, _ = s.Store.GetEntityByID(ctx, recipientID)
	if actorID != nil {
		notification.ActorEntity, _ = s.Store.GetEntityByID(ctx, *actorID)
	}
	s.attachNotificationIdentity(ctx, notification)
	s.Hub.SendToEntity(recipientID, ws.WSMessage{
		Type: "notification.new",
		Data: notification,
	})
	s.pushNotification(ctx, notification)
	return notification, nil
}

// GET /notifications?entity_id=&status=&limit=
func (s *Server) HandleListNotifications(c *gin.Context) {
	entityID, _ := strconv.ParseInt(strings.TrimSpace(c.Query("entity_id")), 10, 64)
	entity, ok := s.resolveActingEntity(c, entityID)
	if !ok {
		return
	}
	status := strings.TrimSpace(c.Query("status"))
	limit := 50
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}
	notifications, err := s.Store.ListNotificationsByEntity(c.Request.Context(), entity.ID, status, limit)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to list notifications")
		return
	}
	for _, notification := range notifications {
		s.attachNotificationIdentity(c.Request.Context(), notification)
	}
	if notifications == nil {
		notifications = []*model.Notification{}
	}
	OK(c, http.StatusOK, notifications)
}

// POST /notifications/:id/read?entity_id=
func (s *Server) HandleMarkNotificationRead(c *gin.Context) {
	notificationID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid notification id")
		return
	}
	entityID, _ := strconv.ParseInt(strings.TrimSpace(c.Query("entity_id")), 10, 64)
	entity, ok := s.resolveActingEntity(c, entityID)
	if !ok {
		return
	}
	notification, err := s.Store.MarkNotificationRead(c.Request.Context(), entity.ID, notificationID)
	if err != nil {
		Fail(c, http.StatusNotFound, "notification not found")
		return
	}
	s.attachNotificationIdentity(c.Request.Context(), notification)
	s.Hub.SendToEntity(entity.ID, ws.WSMessage{
		Type: "notification.read",
		Data: notification,
	})
	OK(c, http.StatusOK, notification)
}

// POST /notifications/read-all?entity_id=
func (s *Server) HandleMarkAllNotificationsRead(c *gin.Context) {
	entityID, _ := strconv.ParseInt(strings.TrimSpace(c.Query("entity_id")), 10, 64)
	entity, ok := s.resolveActingEntity(c, entityID)
	if !ok {
		return
	}
	if err := s.Store.MarkAllNotificationsRead(c.Request.Context(), entity.ID); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to mark notifications read")
		return
	}
	s.Hub.SendToEntity(entity.ID, ws.WSMessage{
		Type: "notification.read_all",
		Data: map[string]any{"entity_id": entity.ID},
	})
	OK(c, http.StatusOK, gin.H{"entity_id": entity.ID, "status": "ok"})
}
