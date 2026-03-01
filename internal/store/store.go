package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
	"github.com/wzfukui/agent-native-im/internal/model"
)

type Store struct {
	DB *bun.DB
}

func New(dbPath string) (*Store, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	dsn := fmt.Sprintf("file:%s?cache=shared&_journal_mode=WAL", dbPath)
	sqldb, err := sql.Open(sqliteshim.ShimName, dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	sqldb.SetMaxOpenConns(1)
	sqldb.SetMaxIdleConns(2)

	db := bun.NewDB(sqldb, sqlitedialect.New())
	s := &Store{DB: db}

	if err := s.createTables(context.Background()); err != nil {
		return nil, fmt.Errorf("create tables: %w", err)
	}

	if err := s.migrate(context.Background()); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return s, nil
}

func (s *Store) createTables(ctx context.Context) error {
	models := []interface{}{
		(*model.User)(nil),
		(*model.Bot)(nil),
		(*model.Conversation)(nil),
		(*model.Message)(nil),
	}
	for _, m := range models {
		if _, err := s.DB.NewCreateTable().Model(m).IfNotExists().Exec(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) migrate(ctx context.Context) error {
	// Add webhook_url column to bots table (idempotent)
	_, err := s.DB.ExecContext(ctx, "ALTER TABLE bots ADD COLUMN webhook_url TEXT DEFAULT ''")
	if err != nil {
		// Ignore "duplicate column" error — column already exists
		if err.Error() != "duplicate column name: webhook_url" {
			return err
		}
	}
	return nil
}

func (s *Store) Close() error {
	return s.DB.Close()
}
