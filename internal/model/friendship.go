package model

import (
	"time"

	"github.com/uptrace/bun"
)

type Friendship struct {
	bun.BaseModel `bun:"table:friendships"`

	ID          int64     `bun:"id,pk,autoincrement" json:"id"`
	EntityLowID int64     `bun:"entity_low_id,notnull" json:"entity_low_id"`
	EntityHighID int64    `bun:"entity_high_id,notnull" json:"entity_high_id"`
	CreatedBy   int64     `bun:"created_by,notnull" json:"created_by"`
	CreatedAt   time.Time `bun:"created_at,nullzero,notnull,default:now()" json:"created_at"`

	EntityLow  *Entity `bun:"rel:belongs-to,join:entity_low_id=id" json:"entity_low,omitempty"`
	EntityHigh *Entity `bun:"rel:belongs-to,join:entity_high_id=id" json:"entity_high,omitempty"`
}
