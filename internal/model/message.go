package model

import (
	"encoding/json"
	"time"

	"github.com/uptrace/bun"
)

type ContentType string

const (
	ContentText     ContentType = "text"
	ContentMarkdown ContentType = "markdown"
	ContentCode     ContentType = "code"
	ContentImage    ContentType = "image"
	ContentArtifact ContentType = "artifact"
	ContentSystem   ContentType = "system"
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
	Type    string              `json:"type"`
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

type Attachment struct {
	Type     string `json:"type"`
	URL      string `json:"url,omitempty"`
	Filename string `json:"filename,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
	Size     int64  `json:"size,omitempty"`
	Content  string `json:"content,omitempty"`
	Language string `json:"language,omitempty"`
}

type Message struct {
	bun.BaseModel `bun:"table:messages"`

	ID             int64         `bun:"id,pk,autoincrement" json:"id"`
	ConversationID int64         `bun:"conversation_id,notnull" json:"conversation_id"`
	SenderID       int64         `bun:"sender_id,notnull" json:"sender_id"`
	StreamID       string        `bun:"stream_id" json:"stream_id,omitempty"`
	ContentType    ContentType   `bun:"content_type,notnull,default:'text'" json:"content_type"`
	Layers         MessageLayers `bun:"layers,type:jsonb" json:"layers"`
	Attachments    []Attachment  `bun:"attachments,type:jsonb" json:"attachments,omitempty"`
	CreatedAt      time.Time     `bun:"created_at,nullzero,notnull,default:now()" json:"created_at"`

	// Computed fields (populated by handler, not stored in DB)
	SenderType string  `bun:"-" json:"sender_type,omitempty"`
	Sender     *Entity `bun:"-" json:"sender,omitempty"`
}
