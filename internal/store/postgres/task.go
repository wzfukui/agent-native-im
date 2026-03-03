package postgres

import (
	"context"
	"time"

	"github.com/wzfukui/agent-native-im/internal/model"
)

func (s *PGStore) CreateTask(ctx context.Context, task *model.Task) error {
	task.CreatedAt = time.Now()
	task.UpdatedAt = time.Now()
	_, err := s.DB.NewInsert().Model(task).Exec(ctx)
	return err
}

func (s *PGStore) GetTask(ctx context.Context, id int64) (*model.Task, error) {
	task := new(model.Task)
	err := s.DB.NewSelect().Model(task).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, err
	}
	return task, nil
}

func (s *PGStore) ListTasksByConversation(ctx context.Context, conversationID int64, status string) ([]*model.Task, error) {
	var tasks []*model.Task
	q := s.DB.NewSelect().Model(&tasks).
		Where("conversation_id = ?", conversationID).
		OrderExpr("sort_order ASC, created_at ASC")
	if status != "" {
		q = q.Where("status = ?", status)
	}
	err := q.Scan(ctx)
	return tasks, err
}

func (s *PGStore) UpdateTask(ctx context.Context, task *model.Task) error {
	task.UpdatedAt = time.Now()
	_, err := s.DB.NewUpdate().Model(task).WherePK().Exec(ctx)
	return err
}

func (s *PGStore) DeleteTask(ctx context.Context, id int64) error {
	_, err := s.DB.NewDelete().Model((*model.Task)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}
