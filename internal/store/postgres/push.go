package postgres

import (
	"context"

	"github.com/wzfukui/agent-native-im/internal/model"
)

func (s *PGStore) CreatePushSubscription(ctx context.Context, sub *model.PushSubscription) error {
	_, err := s.DB.NewInsert().Model(sub).
		On("CONFLICT (entity_id, endpoint) DO UPDATE").
		Set("key_p256dh = EXCLUDED.key_p256dh").
		Set("key_auth = EXCLUDED.key_auth").
		Set("device_id = EXCLUDED.device_id").
		Exec(ctx)
	return err
}

func (s *PGStore) DeletePushSubscription(ctx context.Context, entityID int64, endpoint string) error {
	_, err := s.DB.NewDelete().Model((*model.PushSubscription)(nil)).
		Where("entity_id = ?", entityID).
		Where("endpoint = ?", endpoint).
		Exec(ctx)
	return err
}

func (s *PGStore) GetPushSubscriptionsByEntity(ctx context.Context, entityID int64) ([]*model.PushSubscription, error) {
	var subs []*model.PushSubscription
	err := s.DB.NewSelect().Model(&subs).
		Where("entity_id = ?", entityID).
		Scan(ctx)
	return subs, err
}
