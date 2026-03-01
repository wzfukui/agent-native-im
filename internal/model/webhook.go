package model

import (
	"time"

	"github.com/uptrace/bun"
)

type Webhook struct {
	bun.BaseModel `bun:"table:webhooks"`

	ID        int64     `bun:"id,pk,autoincrement" json:"id"`
	EntityID  int64     `bun:"entity_id,notnull" json:"entity_id"`
	URL       string    `bun:"url,notnull" json:"url"`
	Secret    string    `bun:"secret,notnull" json:"-"`
	Events    []string  `bun:"events,array" json:"events"`
	Status    string    `bun:"status,notnull,default:'active'" json:"status"`
	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:now()" json:"created_at"`
}
