package postgres

import (
	"context"

	"github.com/wzfukui/agent-native-im/internal/model"
)

func (s *PGStore) CreateMessage(ctx context.Context, msg *model.Message) error {
	_, err := s.DB.NewInsert().Model(msg).Exec(ctx)
	return err
}

func (s *PGStore) ListMessages(ctx context.Context, conversationID int64, before int64, limit int) ([]*model.Message, error) {
	var msgs []*model.Message
	q := s.DB.NewSelect().Model(&msgs).
		Where("conversation_id = ?", conversationID).
		OrderExpr("id DESC").
		Limit(limit)

	if before > 0 {
		q = q.Where("id < ?", before)
	}

	err := q.Scan(ctx)
	return msgs, err
}

func (s *PGStore) GetUpdatesForEntity(ctx context.Context, entityID int64, afterID int64) ([]*model.Message, error) {
	var msgs []*model.Message
	err := s.DB.NewSelect().Model(&msgs).
		Where("conversation_id IN (?)",
			s.DB.NewSelect().Model((*model.Participant)(nil)).
				Column("conversation_id").
				Where("entity_id = ?", entityID).
				Where("left_at IS NULL"),
		).
		Where("id > ?", afterID).
		OrderExpr("id ASC").
		Limit(100).
		Scan(ctx)
	return msgs, err
}
