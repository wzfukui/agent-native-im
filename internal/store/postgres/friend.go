package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/uptrace/bun"
	"github.com/wzfukui/agent-native-im/internal/model"
)

func orderedFriendPair(entityA, entityB int64) (int64, int64) {
	if entityA < entityB {
		return entityA, entityB
	}
	return entityB, entityA
}

func (s *PGStore) CreateFriendRequest(ctx context.Context, req *model.FriendRequest) error {
	_, err := s.DB.NewInsert().Model(req).Exec(ctx)
	return err
}

func (s *PGStore) GetFriendRequestByID(ctx context.Context, id int64) (*model.FriendRequest, error) {
	req := new(model.FriendRequest)
	err := s.DB.NewSelect().Model(req).
		Relation("SourceEntity").
		Relation("TargetEntity").
		Where("friend_request.id = ?", id).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return req, nil
}

func (s *PGStore) FindPendingFriendRequest(ctx context.Context, sourceID, targetID int64) (*model.FriendRequest, error) {
	req := new(model.FriendRequest)
	err := s.DB.NewSelect().Model(req).
		Where("source_entity_id = ?", sourceID).
		Where("target_entity_id = ?", targetID).
		Where("status = ?", model.FriendRequestPending).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return req, nil
}

func (s *PGStore) ListFriendRequestsByEntity(ctx context.Context, entityID int64, direction, status string) ([]*model.FriendRequest, error) {
	var reqs []*model.FriendRequest
	q := s.DB.NewSelect().Model(&reqs).
		Relation("SourceEntity").
		Relation("TargetEntity")
	switch direction {
	case "incoming":
		q = q.Where("target_entity_id = ?", entityID)
	case "outgoing":
		q = q.Where("source_entity_id = ?", entityID)
	default:
		q = q.WhereGroup(" AND ", func(sq *bun.SelectQuery) *bun.SelectQuery {
			return sq.Where("source_entity_id = ?", entityID).WhereOr("target_entity_id = ?", entityID)
		})
	}
	if status != "" {
		q = q.Where("friend_request.status = ?", status)
	}
	err := q.OrderExpr("friend_request.created_at DESC").Scan(ctx)
	return reqs, err
}

func (s *PGStore) UpdateFriendRequest(ctx context.Context, req *model.FriendRequest) error {
	req.UpdatedAt = time.Now()
	_, err := s.DB.NewUpdate().Model(req).WherePK().Exec(ctx)
	return err
}

func (s *PGStore) CreateFriendship(ctx context.Context, friendship *model.Friendship) error {
	_, err := s.DB.NewInsert().Model(friendship).
		On("CONFLICT (entity_low_id, entity_high_id) DO NOTHING").
		Exec(ctx)
	return err
}

func (s *PGStore) GetFriendship(ctx context.Context, entityA, entityB int64) (*model.Friendship, error) {
	low, high := orderedFriendPair(entityA, entityB)
	friendship := new(model.Friendship)
	err := s.DB.NewSelect().Model(friendship).
		Where("entity_low_id = ?", low).
		Where("entity_high_id = ?", high).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return friendship, nil
}

func (s *PGStore) ListFriends(ctx context.Context, entityID int64) ([]*model.Entity, error) {
	type row struct {
		model.Entity
	}
	var rows []row
	err := s.DB.NewRaw(`
		SELECT e.*
		FROM friendships f
		JOIN entities e
		  ON e.id = CASE WHEN f.entity_low_id = ? THEN f.entity_high_id ELSE f.entity_low_id END
		WHERE (f.entity_low_id = ? OR f.entity_high_id = ?)
		  AND e.status = 'active'
		ORDER BY e.display_name ASC, e.created_at DESC
	`, entityID, entityID, entityID).Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}
	entities := make([]*model.Entity, 0, len(rows))
	for i := range rows {
		entity := rows[i].Entity
		entities = append(entities, &entity)
	}
	return entities, nil
}

func (s *PGStore) DeleteFriendship(ctx context.Context, entityA, entityB int64) error {
	low, high := orderedFriendPair(entityA, entityB)
	_, err := s.DB.NewDelete().Model((*model.Friendship)(nil)).
		Where("entity_low_id = ?", low).
		Where("entity_high_id = ?", high).
		Exec(ctx)
	return err
}

func isNotFound(err error) bool {
	return err == sql.ErrNoRows
}
