package postgres

import (
	"context"

	"github.com/uptrace/bun"
	"github.com/wzfukui/agent-native-im/internal/model"
)

func (s *PGStore) CreateMessage(ctx context.Context, msg *model.Message) error {
	_, err := s.DB.NewInsert().Model(msg).Exec(ctx)
	return err
}

func (s *PGStore) GetMessageByID(ctx context.Context, id int64) (*model.Message, error) {
	msg := new(model.Message)
	err := s.DB.NewSelect().Model(msg).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, err
	}
	return msg, nil
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

func (s *PGStore) ListMessagesSince(ctx context.Context, conversationID int64, sinceID int64, limit int) ([]*model.Message, error) {
	var msgs []*model.Message
	err := s.DB.NewSelect().Model(&msgs).
		Where("conversation_id = ?", conversationID).
		Where("id > ?", sinceID).
		OrderExpr("id DESC").
		Limit(limit).
		Scan(ctx)
	return msgs, err
}

func (s *PGStore) GlobalSearchMessages(ctx context.Context, entityID int64, query string, limit int, offset int) ([]*model.Message, error) {
	var msgs []*model.Message
	pattern := "%" + query + "%"
	err := s.DB.NewSelect().Model(&msgs).
		Where("conversation_id IN (?)",
			s.DB.NewSelect().Model((*model.Participant)(nil)).
				Column("conversation_id").
				Where("entity_id = ?", entityID).
				Where("left_at IS NULL"),
		).
		Where("revoked_at IS NULL").
		WhereGroup(" AND ", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.
				Where("COALESCE(layers->>'summary', '') ILIKE ?", pattern).
				WhereOr("COALESCE(layers->'data'->>'body', '') ILIKE ?", pattern)
		}).
		OrderExpr("id DESC").
		Limit(limit).
		Offset(offset).
		Scan(ctx)
	return msgs, err
}

func (s *PGStore) SearchMessages(ctx context.Context, conversationID int64, query string, limit int) ([]*model.Message, error) {
	var msgs []*model.Message
	err := s.DB.NewSelect().Model(&msgs).
		Where("conversation_id = ?", conversationID).
		Where("revoked_at IS NULL").
		Where("to_tsvector('simple', COALESCE(layers->>'summary', '')) @@ plainto_tsquery('simple', ?)", query).
		OrderExpr("id DESC").
		Limit(limit).
		Scan(ctx)
	return msgs, err
}

func (s *PGStore) RevokeMessage(ctx context.Context, messageID int64) error {
	_, err := s.DB.NewUpdate().Model((*model.Message)(nil)).
		Set("revoked_at = NOW()").
		Where("id = ?", messageID).
		Where("revoked_at IS NULL").
		Exec(ctx)
	return err
}

func (s *PGStore) UpdateMessage(ctx context.Context, msg *model.Message) error {
	_, err := s.DB.NewUpdate().Model(msg).
		Column("layers").
		Where("id = ?", msg.ID).
		Exec(ctx)
	return err
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
		Where("revoked_at IS NULL").
		OrderExpr("id ASC").
		Limit(200).
		Scan(ctx)
	return msgs, err
}
