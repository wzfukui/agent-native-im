package postgres

import (
	"context"
	"time"

	"github.com/wzfukui/agent-native-im/internal/model"
)

func (s *PGStore) CreateEntity(ctx context.Context, entity *model.Entity) error {
	_, err := s.DB.NewInsert().Model(entity).Exec(ctx)
	return err
}

func (s *PGStore) GetEntityByID(ctx context.Context, id int64) (*model.Entity, error) {
	entity := new(model.Entity)
	err := s.DB.NewSelect().Model(entity).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, err
	}
	return entity, nil
}

func (s *PGStore) GetEntityByName(ctx context.Context, name string, entityType model.EntityType) (*model.Entity, error) {
	entity := new(model.Entity)
	err := s.DB.NewSelect().Model(entity).
		Where("name = ?", name).
		Where("entity_type = ?", entityType).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return entity, nil
}

func (s *PGStore) ListEntitiesByOwner(ctx context.Context, ownerID int64) ([]*model.Entity, error) {
	var entities []*model.Entity
	err := s.DB.NewSelect().Model(&entities).
		Where("owner_id = ?", ownerID).
		OrderExpr("CASE WHEN status = 'active' THEN 0 ELSE 1 END, created_at DESC").
		Scan(ctx)
	return entities, err
}

func (s *PGStore) UpdateEntity(ctx context.Context, entity *model.Entity) error {
	entity.UpdatedAt = time.Now()
	_, err := s.DB.NewUpdate().Model(entity).WherePK().Exec(ctx)
	return err
}

func (s *PGStore) ListAllEntities(ctx context.Context, limit, offset int) ([]*model.Entity, int, error) {
	var entities []*model.Entity
	count, err := s.DB.NewSelect().Model(&entities).
		OrderExpr("created_at DESC").
		Limit(limit).
		Offset(offset).
		ScanAndCount(ctx)
	return entities, count, err
}

func (s *PGStore) DeleteEntity(ctx context.Context, id int64) error {
	_, err := s.DB.NewUpdate().Model((*model.Entity)(nil)).
		Set("status = ?", "disabled").
		Set("updated_at = ?", time.Now()).
		Where("id = ?", id).
		Exec(ctx)
	return err
}

func (s *PGStore) ReactivateEntity(ctx context.Context, id int64) error {
	_, err := s.DB.NewUpdate().Model((*model.Entity)(nil)).
		Set("status = ?", "active").
		Set("updated_at = ?", time.Now()).
		Where("id = ?", id).
		Exec(ctx)
	return err
}
