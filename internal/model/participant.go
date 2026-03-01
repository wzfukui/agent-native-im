package model

import (
	"time"

	"github.com/uptrace/bun"
)

type ParticipantRole string

const (
	RoleOwner    ParticipantRole = "owner"
	RoleAdmin    ParticipantRole = "admin"
	RoleMember   ParticipantRole = "member"
	RoleObserver ParticipantRole = "observer"
)

type Participant struct {
	bun.BaseModel `bun:"table:participants"`

	ID             int64           `bun:"id,pk,autoincrement" json:"id"`
	ConversationID int64           `bun:"conversation_id,notnull" json:"conversation_id"`
	EntityID       int64           `bun:"entity_id,notnull" json:"entity_id"`
	Role           ParticipantRole `bun:"role,notnull,default:'member'" json:"role"`
	JoinedAt       time.Time       `bun:"joined_at,nullzero,notnull,default:now()" json:"joined_at"`
	LeftAt         *time.Time      `bun:"left_at" json:"left_at,omitempty"`

	Entity       *Entity       `bun:"rel:belongs-to,join:entity_id=id" json:"entity,omitempty"`
	Conversation *Conversation `bun:"rel:belongs-to,join:conversation_id=id" json:"-"`
}
