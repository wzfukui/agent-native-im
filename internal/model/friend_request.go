package model

import (
	"time"

	"github.com/uptrace/bun"
)

type FriendRequestStatus string

const (
	FriendRequestPending  FriendRequestStatus = "pending"
	FriendRequestAccepted FriendRequestStatus = "accepted"
	FriendRequestRejected FriendRequestStatus = "rejected"
	FriendRequestCanceled FriendRequestStatus = "canceled"
)

type FriendRequest struct {
	bun.BaseModel `bun:"table:friend_requests"`

	ID             int64               `bun:"id,pk,autoincrement" json:"id"`
	SourceEntityID int64               `bun:"source_entity_id,notnull" json:"source_entity_id"`
	TargetEntityID int64               `bun:"target_entity_id,notnull" json:"target_entity_id"`
	Status         FriendRequestStatus `bun:"status,notnull,default:'pending'" json:"status"`
	Message        string              `bun:"message" json:"message,omitempty"`
	ResolvedBy     *int64              `bun:"resolved_by" json:"resolved_by,omitempty"`
	CreatedAt      time.Time           `bun:"created_at,nullzero,notnull,default:now()" json:"created_at"`
	UpdatedAt      time.Time           `bun:"updated_at,nullzero,notnull,default:now()" json:"updated_at"`

	SourceEntity *Entity `bun:"rel:belongs-to,join:source_entity_id=id" json:"source_entity,omitempty"`
	TargetEntity *Entity `bun:"rel:belongs-to,join:target_entity_id=id" json:"target_entity,omitempty"`
}
