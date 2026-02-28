package model

import (
	"time"

	"github.com/uptrace/bun"
)

type Conversation struct {
	bun.BaseModel `bun:"table:conversations"`

	ID        int64     `bun:"id,pk,autoincrement" json:"id"`
	UserID    int64     `bun:"user_id,notnull" json:"user_id"`
	BotID     int64     `bun:"bot_id,notnull" json:"bot_id"`
	Title     string    `bun:"title" json:"title"`
	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updated_at"`

	User *User `bun:"rel:belongs-to,join:user_id=id" json:"user,omitempty"`
	Bot  *Bot  `bun:"rel:belongs-to,join:bot_id=id" json:"bot,omitempty"`
}
