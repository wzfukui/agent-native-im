package model

import (
	"time"

	"github.com/uptrace/bun"
)

type ConversationMemory struct {
	bun.BaseModel `bun:"table:conversation_memories"`

	ID             int64     `bun:"id,pk,autoincrement" json:"id"`
	ConversationID int64     `bun:"conversation_id,notnull" json:"conversation_id"`
	Key            string    `bun:"key,notnull" json:"key"`
	Content        string    `bun:"content,notnull" json:"content"`
	UpdatedBy      int64     `bun:"updated_by,notnull" json:"updated_by"`
	CreatedAt      time.Time `bun:"created_at,nullzero,notnull,default:now()" json:"created_at"`
	UpdatedAt      time.Time `bun:"updated_at,nullzero,notnull,default:now()" json:"updated_at"`
}
