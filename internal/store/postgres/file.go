package postgres

import (
	"context"

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
