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

type SubscriptionMode string

const (
	SubMentionOnly    SubscriptionMode = "mention_only"
	SubSubscribeAll   SubscriptionMode = "subscribe_all"
	SubMentionWithCtx SubscriptionMode = "mention_with_context"
	SubSubscribeDigest SubscriptionMode = "subscribe_digest"
)

type Participant struct {
	bun.BaseModel `bun:"table:participants"`

	ID               int64            `bun:"id,pk,autoincrement" json:"id"`
	ConversationID   int64            `bun:"conversation_id,notnull" json:"conversation_id"`
	EntityID         int64            `bun:"entity_id,notnull" json:"entity_id"`
	Role             ParticipantRole  `bun:"role,notnull,default:'member'" json:"role"`
	SubscriptionMode SubscriptionMode `bun:"subscription_mode,notnull,default:'mention_only'" json:"subscription_mode"`
	ContextWindow     int              `bun:"context_window,notnull,default:5" json:"context_window"`
	LastReadMessageID int64           `bun:"last_read_message_id,notnull,default:0" json:"last_read_message_id"`
	JoinedAt         time.Time        `bun:"joined_at,nullzero,notnull,default:now()" json:"joined_at"`
	LeftAt           *time.Time       `bun:"left_at" json:"left_at,omitempty"`
	ArchivedAt       *time.Time       `bun:"archived_at" json:"archived_at,omitempty"`
	PinnedAt         *time.Time       `bun:"pinned_at" json:"pinned_at,omitempty"`

	Entity       *Entity       `bun:"rel:belongs-to,join:entity_id=id" json:"entity,omitempty"`
	Conversation *Conversation `bun:"rel:belongs-to,join:conversation_id=id" json:"-"`
}
