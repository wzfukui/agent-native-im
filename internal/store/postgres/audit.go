package postgres

import (
	"context"
	"time"

	"github.com/wzfukui/agent-native-im/internal/model"
)

func (s *PGStore) CreateAuditLog(ctx context.Context, log *model.AuditLog) error {
	_, err := s.DB.NewInsert().Model(log).Exec(ctx)
	return err
}

func (s *PGStore) ListAuditLogs(ctx context.Context, entityID *int64, action string, since string, limit, offset int) ([]*model.AuditLog, int, error) {
	var logs []*model.AuditLog
	q := s.DB.NewSelect().Model(&logs).OrderExpr("created_at DESC")

	if entityID != nil {
		q = q.Where("entity_id = ?", *entityID)
	}
	if action != "" {
		q = q.Where("action = ?", action)
	}
	if since != "" {
		t, err := time.Parse(time.RFC3339, since)
		if err == nil {
			q = q.Where("created_at >= ?", t)
		}
	}

	if limit <= 0 || limit > 100 {
		limit = 50
	}
	q = q.Limit(limit).Offset(offset)

	count, err := q.ScanAndCount(ctx)
	return logs, count, err
}
