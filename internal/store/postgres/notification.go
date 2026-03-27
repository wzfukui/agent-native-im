package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/wzfukui/agent-native-im/internal/model"
)

func (s *PGStore) CreateNotification(ctx context.Context, notification *model.Notification) error {
	_, err := s.DB.NewInsert().Model(notification).Exec(ctx)
	return err
}

func (s *PGStore) ListNotificationsByEntity(ctx context.Context, entityID int64, status string, limit int) ([]*model.Notification, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	var notifications []*model.Notification
	q := s.DB.NewSelect().
		Model(&notifications).
		Relation("RecipientEntity").
		Relation("ActorEntity").
		Where("notification.recipient_entity_id = ?", entityID).
		OrderExpr("notification.created_at DESC").
		Limit(limit)
	if status != "" {
		q = q.Where("notification.status = ?", status)
	}
	err := q.Scan(ctx)
	return notifications, err
}

func (s *PGStore) MarkNotificationRead(ctx context.Context, entityID, notificationID int64) (*model.Notification, error) {
	now := time.Now()
	notification := &model.Notification{
		ID:                notificationID,
		RecipientEntityID: entityID,
		Status:            model.NotificationRead,
		ReadAt:            &now,
		UpdatedAt:         now,
	}
	res, err := s.DB.NewUpdate().
		Model(notification).
		Column("status", "read_at", "updated_at").
		Where("id = ?", notificationID).
		Where("recipient_entity_id = ?", entityID).
		Where("status <> ?", model.NotificationRead).
		Exec(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if rows == 0 {
		current := new(model.Notification)
		err := s.DB.NewSelect().
			Model(current).
			Relation("RecipientEntity").
			Relation("ActorEntity").
			Where("notification.id = ?", notificationID).
			Where("notification.recipient_entity_id = ?", entityID).
			Scan(ctx)
		if err != nil {
			return nil, err
		}
		return current, nil
	}

	current := new(model.Notification)
	err = s.DB.NewSelect().
		Model(current).
		Relation("RecipientEntity").
		Relation("ActorEntity").
		Where("notification.id = ?", notificationID).
		Where("notification.recipient_entity_id = ?", entityID).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return current, nil
}

func (s *PGStore) MarkAllNotificationsRead(ctx context.Context, entityID int64) error {
	now := time.Now()
	_, err := s.DB.NewUpdate().
		Model((*model.Notification)(nil)).
		Set("status = ?", model.NotificationRead).
		Set("read_at = ?", now).
		Set("updated_at = ?", now).
		Where("recipient_entity_id = ?", entityID).
		Where("status <> ?", model.NotificationRead).
		Exec(ctx)
	return err
}

func isNotificationNotFound(err error) bool {
	return err == sql.ErrNoRows
}
