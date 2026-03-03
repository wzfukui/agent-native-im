package postgres

import (
	"context"
	"time"

	"github.com/wzfukui/agent-native-im/internal/model"
)

func (s *PGStore) CreateChangeRequest(ctx context.Context, cr *model.ChangeRequest) error {
	cr.CreatedAt = time.Now()
	_, err := s.DB.NewInsert().Model(cr).Exec(ctx)
	return err
}

func (s *PGStore) ListChangeRequests(ctx context.Context, conversationID int64, status string) ([]*model.ChangeRequest, error) {
	var crs []*model.ChangeRequest
	q := s.DB.NewSelect().Model(&crs).
		Where("conversation_id = ?", conversationID).
		OrderExpr("created_at DESC")
	if status != "" {
		q = q.Where("status = ?", status)
	}
	err := q.Scan(ctx)
	return crs, err
}

func (s *PGStore) GetChangeRequest(ctx context.Context, id int64) (*model.ChangeRequest, error) {
	cr := new(model.ChangeRequest)
	err := s.DB.NewSelect().Model(cr).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, err
	}
	return cr, nil
}

func (s *PGStore) ResolveChangeRequest(ctx context.Context, id int64, approverID int64, approved bool) error {
	status := model.CRRejected
	if approved {
		status = model.CRApproved
	}
	now := time.Now()
	_, err := s.DB.NewUpdate().
		Model((*model.ChangeRequest)(nil)).
		Set("status = ?", status).
		Set("approver_id = ?", approverID).
		Set("resolved_at = ?", now).
		Where("id = ?", id).
		Exec(ctx)
	return err
}
