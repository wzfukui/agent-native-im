package model

import (
	"time"

	"github.com/uptrace/bun"
)

type ChangeRequestStatus string

const (
	CRPending  ChangeRequestStatus = "pending"
	CRApproved ChangeRequestStatus = "approved"
	CRRejected ChangeRequestStatus = "rejected"
)

type ChangeRequest struct {
	bun.BaseModel `bun:"table:conversation_change_requests"`

	ID             int64               `bun:"id,pk,autoincrement" json:"id"`
	ConversationID int64               `bun:"conversation_id,notnull" json:"conversation_id"`
	Field          string              `bun:"field,notnull" json:"field"`
	OldValue       string              `bun:"old_value" json:"old_value"`
	NewValue       string              `bun:"new_value,notnull" json:"new_value"`
	RequesterID    int64               `bun:"requester_id,notnull" json:"requester_id"`
	Status         ChangeRequestStatus `bun:"status,notnull,default:'pending'" json:"status"`
	ApproverID     *int64              `bun:"approver_id" json:"approver_id,omitempty"`
	CreatedAt      time.Time           `bun:"created_at,nullzero,notnull,default:now()" json:"created_at"`
	ResolvedAt     *time.Time          `bun:"resolved_at" json:"resolved_at,omitempty"`

	// Computed fields
	Requester *Entity `bun:"-" json:"requester,omitempty"`
}
