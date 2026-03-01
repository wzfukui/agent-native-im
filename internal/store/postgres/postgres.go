package postgres

import (
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

// PGStore implements store.Store using PostgreSQL via Bun ORM.
type PGStore struct {
	DB *bun.DB
}

// New creates a new PostgreSQL store.
// databaseURL format: "postgres://user:pass@host:port/dbname?sslmode=disable"
func New(databaseURL string) (*PGStore, error) {
	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(databaseURL)))

	sqldb.SetMaxOpenConns(10)
	sqldb.SetMaxIdleConns(5)

	db := bun.NewDB(sqldb, pgdialect.New())

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	return &PGStore{DB: db}, nil
}

func (s *PGStore) Close() error {
	return s.DB.Close()
}
