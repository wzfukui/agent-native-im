package model

import (
	"time"

	"github.com/uptrace/bun"
)

type FileRecord struct {
	bun.BaseModel  `bun:"table:file_records"`
	ID             int64     `bun:"id,pk,autoincrement" json:"id"`
	StoredName     string    `bun:"stored_name,notnull" json:"stored_name"`
	OriginalName   string    `bun:"original_name" json:"original_name"`
	MimeType       string    `bun:"mime_type" json:"mime_type"`
	Size           int64     `bun:"size" json:"size"`
	UploaderID     int64     `bun:"uploader_id,notnull" json:"uploader_id"`
	ConversationID *int64    `bun:"conversation_id" json:"conversation_id,omitempty"`
	CreatedAt      time.Time `bun:"created_at,nullzero,notnull,default:now()" json:"created_at"`
}
