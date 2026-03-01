package postgres

import (
	"context"

	"github.com/wzfukui/agent-native-im/internal/model"
)

func (s *PGStore) AddParticipant(ctx context.Context, p *model.Participant) error {
	_, err := s.DB.NewInsert().Model(p).
		On("CONFLICT (conversation_id, entity_id) DO NOTHING").
		Exec(ctx)
	return err
}

func (s *PGStore) RemoveParticipant(ctx context.Context, conversationID, entityID int64) error {
	_, err := s.DB.NewUpdate().Model((*model.Participant)(nil)).
		Set("left_at = NOW()").
		Where("conversation_id = ?", conversationID).
		Where("entity_id = ?", entityID).
		Exec(ctx)
	return err
}

func (s *PGStore) ListParticipants(ctx context.Context, conversationID int64) ([]*model.Participant, error) {
	var participants []*model.Participant
	err := s.DB.NewSelect().Model(&participants).
		Relation("Entity").
		Where("participant.conversation_id = ?", conversationID).
		Where("participant.left_at IS NULL").
		Scan(ctx)
	return participants, err
}

func (s *PGStore) IsParticipant(ctx context.Context, conversationID, entityID int64) (bool, error) {
	exists, err := s.DB.NewSelect().Model((*model.Participant)(nil)).
		Where("conversation_id = ?", conversationID).
		Where("entity_id = ?", entityID).
		Where("left_at IS NULL").
		Exists(ctx)
	return exists, err
}

func (s *PGStore) GetParticipant(ctx context.Context, conversationID, entityID int64) (*model.Participant, error) {
	p := new(model.Participant)
	err := s.DB.NewSelect().Model(p).
		Where("conversation_id = ?", conversationID).
		Where("entity_id = ?", entityID).
		Where("left_at IS NULL").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (s *PGStore) UpdateSubscription(ctx context.Context, conversationID, entityID int64, mode model.SubscriptionMode) error {
	_, err := s.DB.NewUpdate().Model((*model.Participant)(nil)).
		Set("subscription_mode = ?", mode).
		Where("conversation_id = ?", conversationID).
		Where("entity_id = ?", entityID).
		Where("left_at IS NULL").
		Exec(ctx)
	return err
}

func (s *PGStore) GetConversationIDsByEntity(ctx context.Context, entityID int64) ([]int64, error) {
	var ids []int64
	err := s.DB.NewSelect().Model((*model.Participant)(nil)).
		Column("conversation_id").
		Where("entity_id = ?", entityID).
		Where("left_at IS NULL").
		Scan(ctx, &ids)
	return ids, err
}
