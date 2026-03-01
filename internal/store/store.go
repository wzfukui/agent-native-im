package store

import (
	"context"
	"io"

	"github.com/wzfukui/agent-native-im/internal/model"
)

// Store composes all sub-store interfaces.
type Store interface {
	EntityStore
	CredentialStore
	ConversationStore
	ParticipantStore
	MessageStore
	WebhookStore
	io.Closer
}

type EntityStore interface {
	CreateEntity(ctx context.Context, entity *model.Entity) error
	GetEntityByID(ctx context.Context, id int64) (*model.Entity, error)
	GetEntityByName(ctx context.Context, name string, entityType model.EntityType) (*model.Entity, error)
	ListEntitiesByOwner(ctx context.Context, ownerID int64) ([]*model.Entity, error)
	UpdateEntity(ctx context.Context, entity *model.Entity) error
	DeleteEntity(ctx context.Context, id int64) error
}

type CredentialStore interface {
	CreateCredential(ctx context.Context, cred *model.Credential) error
	GetCredentialsByEntity(ctx context.Context, entityID int64, credType model.CredType) ([]*model.Credential, error)
	GetCredentialByPrefix(ctx context.Context, credType model.CredType, prefix string) ([]*model.Credential, error)
	DeleteCredentialsByEntity(ctx context.Context, entityID int64) error
}

type ConversationStore interface {
	CreateConversation(ctx context.Context, conv *model.Conversation) error
	GetConversation(ctx context.Context, id int64) (*model.Conversation, error)
	ListConversationsByEntity(ctx context.Context, entityID int64) ([]*model.Conversation, error)
	TouchConversation(ctx context.Context, id int64) error
}

type ParticipantStore interface {
	AddParticipant(ctx context.Context, p *model.Participant) error
	RemoveParticipant(ctx context.Context, conversationID, entityID int64) error
	ListParticipants(ctx context.Context, conversationID int64) ([]*model.Participant, error)
	IsParticipant(ctx context.Context, conversationID, entityID int64) (bool, error)
	GetConversationIDsByEntity(ctx context.Context, entityID int64) ([]int64, error)
}

type MessageStore interface {
	CreateMessage(ctx context.Context, msg *model.Message) error
	ListMessages(ctx context.Context, conversationID int64, before int64, limit int) ([]*model.Message, error)
	GetUpdatesForEntity(ctx context.Context, entityID int64, afterID int64) ([]*model.Message, error)
}

type WebhookStore interface {
	CreateWebhook(ctx context.Context, wh *model.Webhook) error
	ListWebhooksByEntity(ctx context.Context, entityID int64) ([]*model.Webhook, error)
	GetWebhooksForConversation(ctx context.Context, conversationID int64, event string) ([]*model.Webhook, error)
	DeleteWebhook(ctx context.Context, id int64) error
}
