package model

import (
	"time"

	"github.com/uptrace/bun"
)

type TaskStatus string

const (
	TaskPending    TaskStatus = "pending"
	TaskInProgress TaskStatus = "in_progress"
	TaskDone       TaskStatus = "done"
	TaskCancelled  TaskStatus = "cancelled"
)

type TaskPriority string

const (
	PriorityLow    TaskPriority = "low"
	PriorityMedium TaskPriority = "medium"
	PriorityHigh   TaskPriority = "high"
)

type Task struct {
	bun.BaseModel `bun:"table:tasks"`

	ID             int64        `bun:"id,pk,autoincrement" json:"id"`
	ConversationID int64        `bun:"conversation_id,notnull" json:"conversation_id"`
	Title          string       `bun:"title,notnull" json:"title"`
	Description    string       `bun:"description,notnull,default:''" json:"description"`
	AssigneeID     *int64       `bun:"assignee_id" json:"assignee_id,omitempty"`
	Status         TaskStatus   `bun:"status,notnull,default:'pending'" json:"status"`
	Priority       TaskPriority `bun:"priority,notnull,default:'medium'" json:"priority"`
	DueDate        *time.Time   `bun:"due_date" json:"due_date,omitempty"`
	ParentTaskID   *int64       `bun:"parent_task_id" json:"parent_task_id,omitempty"`
	SortOrder      int          `bun:"sort_order,notnull,default:0" json:"sort_order"`
	CreatedBy      int64        `bun:"created_by,notnull" json:"created_by"`
	CreatedAt      time.Time    `bun:"created_at,nullzero,notnull,default:now()" json:"created_at"`
	UpdatedAt      time.Time    `bun:"updated_at,nullzero,notnull,default:now()" json:"updated_at"`
	CompletedAt    *time.Time   `bun:"completed_at" json:"completed_at,omitempty"`

	// Computed fields (populated by handler)
	Assignee *Entity `bun:"-" json:"assignee,omitempty"`
	Creator  *Entity `bun:"-" json:"creator,omitempty"`
}
