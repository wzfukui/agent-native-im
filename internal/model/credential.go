package model

import (
	"time"

	"github.com/uptrace/bun"
)

type CredType string

const (
	CredPassword CredType = "password"
	CredAPIKey   CredType = "api_key"
)

type Credential struct {
	bun.BaseModel `bun:"table:credentials"`

	ID         int64      `bun:"id,pk,autoincrement" json:"id"`
	EntityID   int64      `bun:"entity_id,notnull" json:"entity_id"`
	CredType   CredType   `bun:"cred_type,notnull" json:"cred_type"`
	SecretHash string     `bun:"secret_hash,notnull" json:"-"`
	RawPrefix  string     `bun:"raw_prefix,notnull" json:"-"`
	ExpiresAt  *time.Time `bun:"expires_at" json:"expires_at,omitempty"`
	CreatedAt  time.Time  `bun:"created_at,nullzero,notnull,default:now()" json:"created_at"`
}
