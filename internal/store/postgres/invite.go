package postgres

import (
	"context"

	"github.com/wzfukui/agent-native-im/internal/model"
)

func (s *PGStore) CreateInviteLink(ctx context.Context, link *model.InviteLink) error {
	_, err := s.DB.NewInsert().Model(link).Exec(ctx)
	return err
}

func (s *PGStore) GetInviteLinkByCode(ctx context.Context, code string) (*model.InviteLink, error) {
	link := new(model.InviteLink)
	err := s.DB.NewSelect().Model(link).Where("code = ?", code).Scan(ctx)
	if err != nil {
		return nil, err
	}
	return link, nil
}

func (s *PGStore) ListInviteLinks(ctx context.Context, conversationID int64) ([]*model.InviteLink, error) {
	var links []*model.InviteLink
	err := s.DB.NewSelect().Model(&links).
		Where("conversation_id = ?", conversationID).
		OrderExpr("created_at DESC").
		Scan(ctx)
	return links, err
}

func (s *PGStore) IncrementInviteUseCount(ctx context.Context, code string) error {
	_, err := s.DB.NewUpdate().
		Model((*model.InviteLink)(nil)).
		Set("use_count = use_count + 1").
		Where("code = ?", code).
		Exec(ctx)
	return err
}

func (s *PGStore) DeleteInviteLink(ctx context.Context, id int64) error {
	_, err := s.DB.NewDelete().Model((*model.InviteLink)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}
