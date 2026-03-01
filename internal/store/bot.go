package store

import (
	"context"

	"github.com/wzfukui/agent-native-im/internal/model"
)

func (s *Store) CreateBot(ctx context.Context, bot *model.Bot) error {
	_, err := s.DB.NewInsert().Model(bot).Exec(ctx)
	return err
}

func (s *Store) ListBotsByOwner(ctx context.Context, ownerID int64) ([]model.Bot, error) {
	var bots []model.Bot
	err := s.DB.NewSelect().Model(&bots).
		Where("owner_id = ?", ownerID).
		Where("status = ?", "active").
		OrderExpr("created_at DESC").
		Scan(ctx)
	return bots, err
}

func (s *Store) GetBotByToken(ctx context.Context, token string) (*model.Bot, error) {
	bot := new(model.Bot)
	err := s.DB.NewSelect().Model(bot).
		Where("token = ?", token).
		Where("status = ?", "active").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return bot, nil
}

func (s *Store) GetBotByID(ctx context.Context, id int64) (*model.Bot, error) {
	bot := new(model.Bot)
	err := s.DB.NewSelect().Model(bot).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, err
	}
	return bot, nil
}

func (s *Store) UpdateBotWebhookURL(ctx context.Context, id, ownerID int64, webhookURL string) error {
	_, err := s.DB.NewUpdate().Model((*model.Bot)(nil)).
		Set("webhook_url = ?", webhookURL).
		Where("id = ?", id).
		Where("owner_id = ?", ownerID).
		Exec(ctx)
	return err
}

func (s *Store) DeleteBot(ctx context.Context, id, ownerID int64) error {
	_, err := s.DB.NewUpdate().Model((*model.Bot)(nil)).
		Set("status = ?", "disabled").
		Where("id = ?", id).
		Where("owner_id = ?", ownerID).
		Exec(ctx)
	return err
}
