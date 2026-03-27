package postgres

import (
	"context"

	"github.com/wzfukui/agent-native-im/internal/model"
)

func (s *PGStore) CreateBotAccessLink(ctx context.Context, link *model.BotAccessLink) error {
	_, err := s.DB.NewInsert().Model(link).Exec(ctx)
	return err
}

func (s *PGStore) GetBotAccessLinkByID(ctx context.Context, id int64) (*model.BotAccessLink, error) {
	link := new(model.BotAccessLink)
	err := s.DB.NewSelect().Model(link).
		Relation("BotEntity").
		Relation("CreatedByEntity").
		Where("bot_access_link.id = ?", id).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return link, nil
}

func (s *PGStore) GetBotAccessLinkByCode(ctx context.Context, code string) (*model.BotAccessLink, error) {
	link := new(model.BotAccessLink)
	err := s.DB.NewSelect().Model(link).
		Relation("BotEntity").
		Relation("CreatedByEntity").
		Where("bot_access_link.code = ?", code).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return link, nil
}

func (s *PGStore) ListBotAccessLinks(ctx context.Context, botEntityID int64) ([]*model.BotAccessLink, error) {
	var links []*model.BotAccessLink
	err := s.DB.NewSelect().Model(&links).
		Where("bot_entity_id = ?", botEntityID).
		OrderExpr("created_at DESC").
		Scan(ctx)
	return links, err
}

func (s *PGStore) IncrementBotAccessLinkUseCount(ctx context.Context, id int64) error {
	_, err := s.DB.NewUpdate().Model((*model.BotAccessLink)(nil)).
		Set("used_count = used_count + 1").
		Where("id = ?", id).
		Exec(ctx)
	return err
}

func (s *PGStore) DeleteBotAccessLink(ctx context.Context, id int64) error {
	_, err := s.DB.NewDelete().Model((*model.BotAccessLink)(nil)).
		Where("id = ?", id).
		Exec(ctx)
	return err
}
