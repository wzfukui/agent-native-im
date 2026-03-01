package model

import (
	"encoding/json"
	"time"

	"github.com/uptrace/bun"
)

type ConvType string

const (
	ConvDirect  ConvType = "direct"
	ConvGroup   ConvType = "group"
	ConvChannel ConvType = "channel"
)

type Conversation struct {
	bun.BaseModel `bun:"table:conversations"`

	ID        int64           `bun:"id,pk,autoincrement" json:"id"`
	ConvType  ConvType        `bun:"conv_type,notnull,default:'direct'" json:"conv_type"`
	Title     string          `bun:"title" json:"title"`
	Metadata  json.RawMessage `bun:"metadata,type:jsonb,notnull,default:'{}'" json:"metadata,omitempty"`
	CreatedAt time.Time       `bun:"created_at,nullzero,notnull,default:now()" json:"created_at"`
	UpdatedAt time.Time       `bun:"updated_at,nullzero,notnull,default:now()" json:"updated_at"`

	Participants []*Participant `bun:"rel:has-many,join:id=conversation_id" json:"participants,omitempty"`
}
