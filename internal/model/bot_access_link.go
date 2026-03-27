package model

import (
	"time"

	"github.com/uptrace/bun"
)

type BotAccessLink struct {
	bun.BaseModel `bun:"table:bot_access_links"`

	ID                int64      `bun:"id,pk,autoincrement" json:"id"`
	BotEntityID       int64      `bun:"bot_entity_id,notnull" json:"bot_entity_id"`
	Code              string     `bun:"code,notnull" json:"code"`
	Label             string     `bun:"label,notnull,default:''" json:"label,omitempty"`
	ExpiresAt         *time.Time `bun:"expires_at" json:"expires_at,omitempty"`
	MaxUses           int        `bun:"max_uses,notnull,default:0" json:"max_uses"`
	UsedCount         int        `bun:"used_count,notnull,default:0" json:"used_count"`
	CreatedByEntityID int64      `bun:"created_by_entity_id,notnull" json:"created_by_entity_id"`
	CreatedAt         time.Time  `bun:"created_at,nullzero,notnull,default:now()" json:"created_at"`

	BotEntity       *Entity `bun:"rel:belongs-to,join:bot_entity_id=id" json:"bot_entity,omitempty"`
	CreatedByEntity *Entity `bun:"rel:belongs-to,join:created_by_entity_id=id" json:"created_by_entity,omitempty"`
}
