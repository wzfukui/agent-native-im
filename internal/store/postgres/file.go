package postgres

import (
	"context"
	"time"

	"github.com/wzfukui/agent-native-im/internal/model"
)

func (s *PGStore) CreateFileRecord(ctx context.Context, record *model.FileRecord) error {
	_, err := s.DB.NewInsert().Model(record).Exec(ctx)
	return err
}

func (s *PGStore) GetFileRecordByStoredName(ctx context.Context, storedName string) (*model.FileRecord, error) {
	record := new(model.FileRecord)
	err := s.DB.NewSelect().Model(record).Where("stored_name = ?", storedName).Scan(ctx)
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (s *PGStore) ListExpiredFileRecords(ctx context.Context, olderThan time.Time, limit int) ([]*model.FileRecord, error) {
	var records []*model.FileRecord
	err := s.DB.NewSelect().Model(&records).
		Where("created_at < ?", olderThan).
		Limit(limit).
		Scan(ctx)
	return records, err
}

func (s *PGStore) DeleteFileRecord(ctx context.Context, id int64) error {
	_, err := s.DB.NewDelete().Model((*model.FileRecord)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}
