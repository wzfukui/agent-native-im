package model

import (
	"encoding/json"
	"time"

	"github.com/uptrace/bun"
)

type StatusLayer struct {
	Phase    string  `json:"phase"`
	Progress float64 `json:"progress"`
	Text     string  `json:"text,omitempty"`
}

type InteractionOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type Interaction struct {
	Type    string              `json:"type"` // approval, choice, form
	Prompt  string              `json:"prompt,omitempty"`
	Options []InteractionOption `json:"options,omitempty"`
}

type MessageLayers struct {
	Thinking    string          `json:"thinking,omitempty"`
	Status      *StatusLayer    `json:"status,omitempty"`
	Data        json.RawMessage `json:"data,omitempty"`
	Summary     string          `json:"summary,omitempty"`
	Interaction *Interaction    `json:"interaction,omitempty"`
}

type Message struct {
	bun.BaseModel `bun:"table:messages"`

	ID             int64         `bun:"id,pk,autoincrement" json:"id"`
	ConversationID int64         `bun:"conversation_id,notnull" json:"conversation_id"`
	StreamID       string        `bun:"stream_id" json:"stream_id,omitempty"`
	SenderType     string        `bun:"sender_type,notnull" json:"sender_type"` // "user" or "bot"
	SenderID       int64         `bun:"sender_id,notnull" json:"sender_id"`
	Layers         MessageLayers `bun:"layers,type:jsonb" json:"layers"`
	CreatedAt      time.Time     `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
}
