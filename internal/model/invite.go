package model

import "time"

type InviteLink struct {
	ID             int64      `bun:"id,pk,autoincrement" json:"id"`
	ConversationID int64      `bun:"conversation_id,notnull" json:"conversation_id"`
	Code           string     `bun:"code,notnull,unique" json:"code"`
	CreatedBy      int64      `bun:"created_by,notnull" json:"created_by"`
	MaxUses        int        `bun:"max_uses,notnull,default:0" json:"max_uses"`
	UseCount       int        `bun:"use_count,notnull,default:0" json:"use_count"`
	ExpiresAt      *time.Time `bun:"expires_at" json:"expires_at,omitempty"`
	CreatedAt      time.Time  `bun:"created_at,notnull,default:current_timestamp" json:"created_at"`
}
