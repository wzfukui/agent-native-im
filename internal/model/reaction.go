package model

import (
	"time"

	"github.com/uptrace/bun"
)

type Reaction struct {
	bun.BaseModel `bun:"table:reactions"`

	ID        int64     `bun:"id,pk,autoincrement" json:"id"`
	MessageID int64     `bun:"message_id,notnull" json:"message_id"`
	EntityID  int64     `bun:"entity_id,notnull" json:"entity_id"`
	Emoji     string    `bun:"emoji,notnull" json:"emoji"`
	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:now()" json:"created_at"`
}

// ReactionSummary is the aggregated view returned to clients.
type ReactionSummary struct {
	Emoji     string  `json:"emoji"`
	Count     int     `json:"count"`
	EntityIDs []int64 `json:"entity_ids"`
}
