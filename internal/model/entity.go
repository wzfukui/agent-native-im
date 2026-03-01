package model

import (
	"encoding/json"
	"time"

	"github.com/uptrace/bun"
)

type EntityType string

const (
	EntityUser    EntityType = "user"
	EntityBot     EntityType = "bot"
	EntityService EntityType = "service"
)

type Entity struct {
	bun.BaseModel `bun:"table:entities"`

	ID          int64           `bun:"id,pk,autoincrement" json:"id"`
	EntityType  EntityType      `bun:"entity_type,notnull" json:"entity_type"`
	Name        string          `bun:"name,notnull" json:"name"`
	DisplayName string          `bun:"display_name,notnull" json:"display_name"`
	AvatarURL   string          `bun:"avatar_url,notnull" json:"avatar_url,omitempty"`
	Status      string          `bun:"status,notnull,default:'active'" json:"status"`
	Metadata    json.RawMessage `bun:"metadata,type:jsonb,notnull,default:'{}'" json:"metadata,omitempty"`
	OwnerID     *int64          `bun:"owner_id" json:"owner_id,omitempty"`
	CreatedAt   time.Time       `bun:"created_at,nullzero,notnull,default:now()" json:"created_at"`
	UpdatedAt   time.Time       `bun:"updated_at,nullzero,notnull,default:now()" json:"updated_at"`

	Owner *Entity `bun:"rel:belongs-to,join:owner_id=id" json:"owner,omitempty"`
}
