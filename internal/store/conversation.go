package store

import (
	"context"
	"time"

	"github.com/wzfukui/agent-native-im/internal/model"
)

func (s *Store) CreateConversation(ctx context.Context, conv *model.Conversation) error {
	conv.UpdatedAt = time.Now()
	_, err := s.DB.NewInsert().Model(conv).Exec(ctx)
	return err
}

func (s *Store) ListConversationsByUser(ctx context.Context, userID int64) ([]model.Conversation, error) {
	var convs []model.Conversation
	err := s.DB.NewSelect().Model(&convs).
		Relation("Bot").
		Where("conversation.user_id = ?", userID).
		OrderExpr("conversation.updated_at DESC").
		Scan(ctx)
	return convs, err
}

func (s *Store) ListConversationsByBot(ctx context.Context, botID int64) ([]model.Conversation, error) {
	var convs []model.Conversation
	err := s.DB.NewSelect().Model(&convs).
		Where("bot_id = ?", botID).
		OrderExpr("updated_at DESC").
		Scan(ctx)
	return convs, err
}

func (s *Store) GetConversation(ctx context.Context, id int64) (*model.Conversation, error) {
	conv := new(model.Conversation)
	err := s.DB.NewSelect().Model(conv).
		Relation("Bot").
		Relation("User").
		Where("conversation.id = ?", id).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return conv, nil
}

func (s *Store) TouchConversation(ctx context.Context, id int64) error {
	_, err := s.DB.NewUpdate().Model((*model.Conversation)(nil)).
		Set("updated_at = ?", time.Now()).
		Where("id = ?", id).
		Exec(ctx)
	return err
}

func (s *Store) GetConversationIDsByBot(ctx context.Context, botID int64) ([]int64, error) {
	var ids []int64
	err := s.DB.NewSelect().Model((*model.Conversation)(nil)).
		Column("id").
		Where("bot_id = ?", botID).
		Scan(ctx, &ids)
	return ids, err
}

func (s *Store) GetConversationIDsByUser(ctx context.Context, userID int64) ([]int64, error) {
	var ids []int64
	err := s.DB.NewSelect().Model((*model.Conversation)(nil)).
		Column("id").
		Where("user_id = ?", userID).
		Scan(ctx, &ids)
	return ids, err
}
