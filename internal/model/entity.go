package model

import (
	"encoding/json"
	"time"

	"github.com/uptrace/bun"
)

type EntityType string

const (
	EntityUser    EntityType = "user"
	EntityBot     EntityType = "bot"
	EntityService EntityType = "service"
)

type Entity struct {
	bun.BaseModel `bun:"table:entities"`

	ID                 int64           `bun:"id,pk,autoincrement" json:"id"`
	PublicID           string          `bun:"public_id,notnull" json:"public_id,omitempty"`
	BotID              string          `bun:"bot_id,nullzero" json:"bot_id,omitempty"`
	EntityType         EntityType      `bun:"entity_type,notnull" json:"entity_type"`
	Name               string          `bun:"name,notnull" json:"name"`
	Email              string          `bun:"email" json:"email,omitempty"`
	DisplayName        string          `bun:"display_name,notnull" json:"display_name"`
	AvatarURL          string          `bun:"avatar_url,notnull" json:"avatar_url,omitempty"`
	Status             string          `bun:"status,notnull,default:'active'" json:"status"`
	Discoverability    string          `bun:"discoverability,notnull,default:'private'" json:"discoverability,omitempty"`
	AllowNonFriendChat bool            `bun:"allow_non_friend_chat,notnull,default:false" json:"allow_non_friend_chat"`
	Metadata           json.RawMessage `bun:"metadata,type:jsonb,notnull,default:'{}'" json:"metadata,omitempty"`
	OwnerID            *int64          `bun:"owner_id" json:"owner_id,omitempty"`
	CreatedAt          time.Time       `bun:"created_at,nullzero,notnull,default:now()" json:"created_at"`
	UpdatedAt          time.Time       `bun:"updated_at,nullzero,notnull,default:now()" json:"updated_at"`

	Owner *Entity `bun:"rel:belongs-to,join:owner_id=id" json:"owner,omitempty"`
}
