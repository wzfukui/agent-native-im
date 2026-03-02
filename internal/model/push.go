package model

import (
	"time"

	"github.com/uptrace/bun"
)

type PushSubscription struct {
	bun.BaseModel `bun:"table:push_subscriptions"`

	ID        int64     `bun:"id,pk,autoincrement" json:"id"`
	EntityID  int64     `bun:"entity_id,notnull" json:"entity_id"`
	DeviceID  string    `bun:"device_id" json:"device_id"`
	Endpoint  string    `bun:"endpoint,notnull" json:"endpoint"`
	KeyP256DH string    `bun:"key_p256dh,notnull" json:"-"`
	KeyAuth   string    `bun:"key_auth,notnull" json:"-"`
	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:now()" json:"created_at"`
}
