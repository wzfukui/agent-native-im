package postgres

import (
	"context"

	"github.com/wzfukui/agent-native-im/internal/model"
)

func (s *PGStore) CreateWebhook(ctx context.Context, wh *model.Webhook) error {
	_, err := s.DB.NewInsert().Model(wh).Exec(ctx)
	return err
}

func (s *PGStore) GetWebhookByID(ctx context.Context, id int64) (*model.Webhook, error) {
	wh := new(model.Webhook)
	err := s.DB.NewSelect().Model(wh).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, err
	}
	return wh, nil
}

func (s *PGStore) ListWebhooksByEntity(ctx context.Context, entityID int64) ([]*model.Webhook, error) {
	var webhooks []*model.Webhook
	err := s.DB.NewSelect().Model(&webhooks).
		Where("entity_id = ?", entityID).
		Where("status = ?", "active").
		Scan(ctx)
	return webhooks, err
}

func (s *PGStore) GetWebhooksForConversation(ctx context.Context, conversationID int64, event string) ([]*model.Webhook, error) {
	var webhooks []*model.Webhook
	err := s.DB.NewSelect().Model(&webhooks).
		Where("status = ?", "active").
		Where("? = ANY(events)", event).
		Where("entity_id IN (?)",
			s.DB.NewSelect().Model((*model.Participant)(nil)).
				Column("entity_id").
				Where("conversation_id = ?", conversationID).
				Where("left_at IS NULL"),
		).
		Scan(ctx)
	return webhooks, err
}

func (s *PGStore) DeleteWebhook(ctx context.Context, id int64) error {
	_, err := s.DB.NewUpdate().Model((*model.Webhook)(nil)).
		Set("status = ?", "disabled").
		Where("id = ?", id).
		Exec(ctx)
	return err
}
