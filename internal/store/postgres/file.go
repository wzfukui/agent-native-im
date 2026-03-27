package postgres

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/uptrace/bun"
	"github.com/wzfukui/agent-native-im/internal/model"
)

func avatarStoredNameFromURL(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	parsed, err := url.Parse(value)
	if err == nil && parsed.Path != "" {
		value = parsed.Path
	}
	switch {
	case strings.HasPrefix(value, "/files/"):
		return strings.TrimPrefix(value, "/files/")
	case strings.HasPrefix(value, "/avatar-files/"):
		return strings.TrimPrefix(value, "/avatar-files/")
	case strings.HasPrefix(value, "/avatars/"):
		return strings.TrimPrefix(value, "/avatars/")
	default:
		return ""
	}
}

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

func (s *PGStore) ListAllStoredNames(ctx context.Context) ([]string, error) {
	var names []string
	err := s.DB.NewSelect().Model((*model.FileRecord)(nil)).
		Column("stored_name").
		Scan(ctx, &names)
	return names, err
}

func (s *PGStore) ListReferencedAvatarStoredNames(ctx context.Context) ([]string, error) {
	var urls []string
	if err := s.DB.NewSelect().
		Model((*model.Entity)(nil)).
		Column("avatar_url").
		Where("avatar_url LIKE '/files/%'").
		Scan(ctx, &urls); err != nil {
		return nil, err
	}

	names := make([]string, 0, len(urls))
	for _, url := range urls {
		name := avatarStoredNameFromURL(url)
		if name != "" {
			names = append(names, name)
		}
	}
	return names, nil
}

func (s *PGStore) IsAvatarStoredNameReferenced(ctx context.Context, storedName string) (bool, error) {
	if storedName == "" {
		return false, nil
	}
	return s.DB.NewSelect().
		Model((*model.Entity)(nil)).
		Where("avatar_url IN (?)", bun.In([]string{
			"/files/" + storedName,
			"/avatar-files/" + storedName,
			"/avatar-files/" + storedName + "?v=1",
			"/avatars/" + storedName,
		})).
		Exists(ctx)
}
