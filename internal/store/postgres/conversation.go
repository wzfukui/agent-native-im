package postgres

import (
	"context"
	"time"

	"github.com/wzfukui/agent-native-im/internal/model"
)

func (s *PGStore) CreateConversation(ctx context.Context, conv *model.Conversation) error {
	_, err := s.DB.NewInsert().Model(conv).Exec(ctx)
	return err
}

func (s *PGStore) GetConversation(ctx context.Context, id int64) (*model.Conversation, error) {
	conv := new(model.Conversation)
	err := s.DB.NewSelect().Model(conv).
		Relation("Participants").
		Relation("Participants.Entity").
		Where("conversation.id = ?", id).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return conv, nil
}

func (s *PGStore) ListConversationsByEntity(ctx context.Context, entityID int64) ([]*model.Conversation, error) {
	var convs []*model.Conversation
	err := s.DB.NewSelect().Model(&convs).
		Join("JOIN participants AS p ON p.conversation_id = conversation.id").
		Where("p.entity_id = ?", entityID).
		Where("p.left_at IS NULL").
		Relation("Participants").
		Relation("Participants.Entity").
		OrderExpr("conversation.updated_at DESC").
		Scan(ctx)
	return convs, err
}

func (s *PGStore) TouchConversation(ctx context.Context, id int64) error {
	_, err := s.DB.NewUpdate().Model((*model.Conversation)(nil)).
		Set("updated_at = ?", time.Now()).
		Where("id = ?", id).
		Exec(ctx)
	return err
}

func (s *PGStore) UpdateConversation(ctx context.Context, conv *model.Conversation) error {
	_, err := s.DB.NewUpdate().Model(conv).
		Where("id = ?", conv.ID).
		Exec(ctx)
	return err
}
