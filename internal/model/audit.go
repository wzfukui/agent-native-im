package model

import (
	"encoding/json"
	"time"

	"github.com/uptrace/bun"
)

type AuditLog struct {
	bun.BaseModel `bun:"table:audit_logs"`

	ID           int64           `bun:"id,pk,autoincrement" json:"id"`
	EntityID     *int64          `bun:"entity_id" json:"entity_id,omitempty"`
	Action       string          `bun:"action,notnull" json:"action"`
	ResourceType string          `bun:"resource_type" json:"resource_type,omitempty"`
	ResourceID   *int64          `bun:"resource_id" json:"resource_id,omitempty"`
	Details      json.RawMessage `bun:"details,type:jsonb,notnull,default:'{}'" json:"details"`
	IPAddress    string          `bun:"ip_address" json:"ip_address,omitempty"`
	CreatedAt    time.Time       `bun:"created_at,nullzero,notnull,default:now()" json:"created_at"`
}
