package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/wzfukui/agent-native-im/internal/model"
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
