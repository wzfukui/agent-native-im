package postgres

import (
	"context"
	"time"

	"github.com/wzfukui/agent-native-im/internal/model"
)

func (s *PGStore) CreateMemory(ctx context.Context, mem *model.ConversationMemory) error {
	mem.CreatedAt = time.Now()
	mem.UpdatedAt = time.Now()
	_, err := s.DB.NewInsert().Model(mem).Exec(ctx)
	return err
}

func (s *PGStore) GetMemory(ctx context.Context, conversationID int64, key string) (*model.ConversationMemory, error) {
	mem := new(model.ConversationMemory)
	err := s.DB.NewSelect().Model(mem).
		Where("conversation_id = ?", conversationID).
		Where("key = ?", key).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return mem, nil
}

func (s *PGStore) ListMemories(ctx context.Context, conversationID int64) ([]*model.ConversationMemory, error) {
	var mems []*model.ConversationMemory
	err := s.DB.NewSelect().Model(&mems).
		Where("conversation_id = ?", conversationID).
		OrderExpr("key ASC").
		Scan(ctx)
	return mems, err
}

func (s *PGStore) UpsertMemory(ctx context.Context, mem *model.ConversationMemory) error {
	mem.UpdatedAt = time.Now()
	_, err := s.DB.NewInsert().Model(mem).
		On("CONFLICT (conversation_id, key) DO UPDATE").
		Set("content = EXCLUDED.content").
		Set("updated_by = EXCLUDED.updated_by").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	return err
}

func (s *PGStore) DeleteMemory(ctx context.Context, id int64) error {
	_, err := s.DB.NewDelete().Model((*model.ConversationMemory)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}
