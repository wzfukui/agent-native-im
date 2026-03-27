package model

import (
	"encoding/json"
	"time"

	"github.com/uptrace/bun"
)

type NotificationStatus string

const (
	NotificationUnread NotificationStatus = "unread"
	NotificationRead   NotificationStatus = "read"
)

type Notification struct {
	bun.BaseModel `bun:"table:notifications"`

	ID                int64              `bun:"id,pk,autoincrement" json:"id"`
	RecipientEntityID int64              `bun:"recipient_entity_id,notnull" json:"recipient_entity_id"`
	ActorEntityID     *int64             `bun:"actor_entity_id" json:"actor_entity_id,omitempty"`
	Kind              string             `bun:"kind,notnull" json:"kind"`
	Status            NotificationStatus `bun:"status,notnull,default:'unread'" json:"status"`
	Title             string             `bun:"title,notnull,default:''" json:"title"`
	Body              string             `bun:"body,notnull,default:''" json:"body"`
	Data              json.RawMessage    `bun:"data,type:jsonb,notnull,default:'{}'" json:"data,omitempty"`
	ReadAt            *time.Time         `bun:"read_at" json:"read_at,omitempty"`
	CreatedAt         time.Time          `bun:"created_at,nullzero,notnull,default:now()" json:"created_at"`
	UpdatedAt         time.Time          `bun:"updated_at,nullzero,notnull,default:now()" json:"updated_at"`

	RecipientEntity *Entity `bun:"rel:belongs-to,join:recipient_entity_id=id" json:"recipient_entity,omitempty"`
	ActorEntity     *Entity `bun:"rel:belongs-to,join:actor_entity_id=id" json:"actor_entity,omitempty"`
}
