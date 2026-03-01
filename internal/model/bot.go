package model

import (
	"time"

	"github.com/uptrace/bun"
)

type Bot struct {
	bun.BaseModel `bun:"table:bots"`

	ID         int64     `bun:"id,pk,autoincrement" json:"id"`
	OwnerID    int64     `bun:"owner_id,notnull" json:"owner_id"`
	Name       string    `bun:"name,notnull" json:"name"`
	Token      string    `bun:"token,unique,notnull" json:"-"`
	Status     string    `bun:"status,notnull,default:'active'" json:"status"`
	WebhookURL string    `bun:"webhook_url" json:"webhook_url,omitempty"`
	CreatedAt  time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`

	Owner *User `bun:"rel:belongs-to,join:owner_id=id" json:"owner,omitempty"`
}
