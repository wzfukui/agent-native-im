package store

import (
	"context"

	"github.com/wzfukui/agent-native-im/internal/model"
)

func (s *Store) CreateMessage(ctx context.Context, msg *model.Message) error {
	_, err := s.DB.NewInsert().Model(msg).Exec(ctx)
	return err
}

func (s *Store) ListMessages(ctx context.Context, convID int64, before int64, limit int) ([]model.Message, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	var msgs []model.Message
	q := s.DB.NewSelect().Model(&msgs).
		Where("conversation_id = ?", convID).
		OrderExpr("id DESC").
		Limit(limit)

	if before > 0 {
		q = q.Where("id < ?", before)
	}

	err := q.Scan(ctx)
	if msgs == nil {
		msgs = []model.Message{}
	}
	return msgs, err
}

func (s *Store) GetUpdatesForBot(ctx context.Context, botID int64, afterID int64) ([]model.Message, error) {
	var msgs []model.Message
	err := s.DB.NewSelect().Model(&msgs).
		Where("conversation_id IN (?)",
			s.DB.NewSelect().Model((*model.Conversation)(nil)).
				Column("id").
				Where("bot_id = ?", botID)).
		Where("id > ?", afterID).
		OrderExpr("id ASC").
		Limit(100).
		Scan(ctx)
	return msgs, err
}
