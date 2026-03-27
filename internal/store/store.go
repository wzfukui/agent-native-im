package store

import (
	"context"
	"database/sql"
	"io"
	"time"

	"github.com/wzfukui/agent-native-im/internal/model"
)

// Store composes all sub-store interfaces.
type Store interface {
	EntityStore
	CredentialStore
	ConversationStore
	ParticipantStore
	MessageStore
	ReactionStore
	WebhookStore
	PushStore
	InviteStore
	FriendStore
	BotAccessStore
	NotificationStore
	TaskStore
	MemoryStore
	ChangeRequestStore
	AuditStore
	StatsStore
	FileRecordStore
	io.Closer
}

type FileRecordStore interface {
	CreateFileRecord(ctx context.Context, record *model.FileRecord) error
	GetFileRecordByStoredName(ctx context.Context, storedName string) (*model.FileRecord, error)
	ListExpiredFileRecords(ctx context.Context, olderThan time.Time, limit int) ([]*model.FileRecord, error)
	DeleteFileRecord(ctx context.Context, id int64) error
	ListAllStoredNames(ctx context.Context) ([]string, error)
	ListReferencedAvatarStoredNames(ctx context.Context) ([]string, error)
	IsAvatarStoredNameReferenced(ctx context.Context, storedName string) (bool, error)
}

type StatsStore interface {
	GetSystemStats(ctx context.Context) (map[string]interface{}, error)
	DBPoolStats() sql.DBStats
}

type PushStore interface {
	CreatePushSubscription(ctx context.Context, sub *model.PushSubscription) error
	DeletePushSubscription(ctx context.Context, entityID int64, endpoint string) error
	GetPushSubscriptionsByEntity(ctx context.Context, entityID int64) ([]*model.PushSubscription, error)
}

type EntityStore interface {
	CreateEntity(ctx context.Context, entity *model.Entity) error
	GetEntityByID(ctx context.Context, id int64) (*model.Entity, error)
	GetEntityByPublicID(ctx context.Context, publicID string) (*model.Entity, error)
	GetEntityByBotID(ctx context.Context, botID string) (*model.Entity, error)
	GetEntitiesByIDs(ctx context.Context, ids []int64) ([]*model.Entity, error)
	GetEntityByName(ctx context.Context, name string, entityType model.EntityType) (*model.Entity, error)
	GetEntityByEmail(ctx context.Context, email string) (*model.Entity, error)
	ListEntitiesByOwner(ctx context.Context, ownerID int64) ([]*model.Entity, error)
	ListAllEntities(ctx context.Context, limit, offset int) ([]*model.Entity, int, error)
	SearchEntitiesByCapability(ctx context.Context, capability string) ([]*model.Entity, error)
	SearchDiscoverableEntities(ctx context.Context, query string, limit int, excludeEntityID int64) ([]*model.Entity, error)
	UpdateEntity(ctx context.Context, entity *model.Entity) error
	DeleteEntity(ctx context.Context, id int64) error
	ReactivateEntity(ctx context.Context, id int64) error
}

type FriendStore interface {
	CreateFriendRequest(ctx context.Context, req *model.FriendRequest) error
	GetFriendRequestByID(ctx context.Context, id int64) (*model.FriendRequest, error)
	FindPendingFriendRequest(ctx context.Context, sourceID, targetID int64) (*model.FriendRequest, error)
	ListFriendRequestsByEntity(ctx context.Context, entityID int64, direction, status string) ([]*model.FriendRequest, error)
	UpdateFriendRequest(ctx context.Context, req *model.FriendRequest) error
	CreateFriendship(ctx context.Context, friendship *model.Friendship) error
	GetFriendship(ctx context.Context, entityA, entityB int64) (*model.Friendship, error)
	ListFriends(ctx context.Context, entityID int64) ([]*model.Entity, error)
	DeleteFriendship(ctx context.Context, entityA, entityB int64) error
}

type BotAccessStore interface {
	CreateBotAccessLink(ctx context.Context, link *model.BotAccessLink) error
	GetBotAccessLinkByID(ctx context.Context, id int64) (*model.BotAccessLink, error)
	GetBotAccessLinkByCode(ctx context.Context, code string) (*model.BotAccessLink, error)
	ListBotAccessLinks(ctx context.Context, botEntityID int64) ([]*model.BotAccessLink, error)
	IncrementBotAccessLinkUseCount(ctx context.Context, id int64) error
	DeleteBotAccessLink(ctx context.Context, id int64) error
}

type NotificationStore interface {
	CreateNotification(ctx context.Context, notification *model.Notification) error
	ListNotificationsByEntity(ctx context.Context, entityID int64, status string, limit int) ([]*model.Notification, error)
	MarkNotificationRead(ctx context.Context, entityID, notificationID int64) (*model.Notification, error)
	MarkAllNotificationsRead(ctx context.Context, entityID int64) error
}

type CredentialStore interface {
	CreateCredential(ctx context.Context, cred *model.Credential) error
	GetCredentialsByEntity(ctx context.Context, entityID int64, credType model.CredType) ([]*model.Credential, error)
	GetCredentialByPrefix(ctx context.Context, credType model.CredType, prefix string) ([]*model.Credential, error)
	UpdateCredential(ctx context.Context, cred *model.Credential) error
	DeleteCredentialsByEntity(ctx context.Context, entityID int64) error
	DeleteCredential(ctx context.Context, credentialID int64) error
	DeleteCredentialsByType(ctx context.Context, entityID int64, credType model.CredType) error
	DeleteCredentialsByTypeExceptHash(ctx context.Context, entityID int64, credType model.CredType, keepSecretHash string) error
}

type ConversationStore interface {
	CreateConversation(ctx context.Context, conv *model.Conversation) error
	GetConversation(ctx context.Context, id int64) (*model.Conversation, error)
	GetConversationByPublicID(ctx context.Context, publicID string) (*model.Conversation, error)
	ListConversationsByEntity(ctx context.Context, entityID int64) ([]*model.Conversation, error)
	ListArchivedConversationsByEntity(ctx context.Context, entityID int64) ([]*model.Conversation, error)
	ListAllConversations(ctx context.Context, limit, offset int) ([]*model.Conversation, int, error)
	TouchConversation(ctx context.Context, id int64) error
	UpdateConversation(ctx context.Context, conv *model.Conversation) error
}

type ParticipantStore interface {
	AddParticipant(ctx context.Context, p *model.Participant) error
	RemoveParticipant(ctx context.Context, conversationID, entityID int64) error
	ListParticipants(ctx context.Context, conversationID int64) ([]*model.Participant, error)
	IsParticipant(ctx context.Context, conversationID, entityID int64) (bool, error)
	GetConversationIDsByEntity(ctx context.Context, entityID int64) ([]int64, error)
	GetParticipant(ctx context.Context, conversationID, entityID int64) (*model.Participant, error)
	UpdateSubscription(ctx context.Context, conversationID, entityID int64, mode model.SubscriptionMode) error
	UpdateSubscriptionWithContext(ctx context.Context, conversationID, entityID int64, mode model.SubscriptionMode, contextWindow int) error
	MarkAsRead(ctx context.Context, conversationID, entityID, messageID int64) error
	GetUnreadCounts(ctx context.Context, entityID int64) (map[int64]int, error)
	UpdateParticipantRole(ctx context.Context, conversationID, entityID int64, role model.ParticipantRole) error
	ArchiveConversation(ctx context.Context, conversationID, entityID int64) error
	UnarchiveConversation(ctx context.Context, conversationID, entityID int64) error
	PinConversation(ctx context.Context, conversationID, entityID int64) error
	UnpinConversation(ctx context.Context, conversationID, entityID int64) error
}

type MessageStore interface {
	CreateMessage(ctx context.Context, msg *model.Message) error
	GetMessageByID(ctx context.Context, id int64) (*model.Message, error)
	ListMessages(ctx context.Context, conversationID int64, before int64, limit int) ([]*model.Message, error)
	ListMessagesSince(ctx context.Context, conversationID int64, sinceID int64, limit int) ([]*model.Message, error)
	SearchMessages(ctx context.Context, conversationID int64, query string, limit int) ([]*model.Message, error)
	GlobalSearchMessages(ctx context.Context, entityID int64, query string, limit int, offset int) ([]*model.Message, error)
	GetUpdatesForEntity(ctx context.Context, entityID int64, afterID int64) ([]*model.Message, error)
	RevokeMessage(ctx context.Context, messageID int64) error
	UpdateMessage(ctx context.Context, msg *model.Message) error
}

type ReactionStore interface {
	AddReaction(ctx context.Context, r *model.Reaction) error
	RemoveReaction(ctx context.Context, messageID, entityID int64, emoji string) error
	GetReactionsByMessages(ctx context.Context, messageIDs []int64) (map[int64][]model.ReactionSummary, error)
}

type InviteStore interface {
	CreateInviteLink(ctx context.Context, link *model.InviteLink) error
	GetInviteLinkByCode(ctx context.Context, code string) (*model.InviteLink, error)
	GetInviteLinkByID(ctx context.Context, id int64) (*model.InviteLink, error)
	ListInviteLinks(ctx context.Context, conversationID int64) ([]*model.InviteLink, error)
	IncrementInviteUseCount(ctx context.Context, code string) error
	DeleteInviteLink(ctx context.Context, id int64) error
}

type WebhookStore interface {
	CreateWebhook(ctx context.Context, wh *model.Webhook) error
	GetWebhookByID(ctx context.Context, id int64) (*model.Webhook, error)
	ListWebhooksByEntity(ctx context.Context, entityID int64) ([]*model.Webhook, error)
	GetWebhooksForConversation(ctx context.Context, conversationID int64, event string) ([]*model.Webhook, error)
	DeleteWebhook(ctx context.Context, id int64) error
}

type TaskStore interface {
	CreateTask(ctx context.Context, task *model.Task) error
	GetTask(ctx context.Context, id int64) (*model.Task, error)
	ListTasksByConversation(ctx context.Context, conversationID int64, status string) ([]*model.Task, error)
	UpdateTask(ctx context.Context, task *model.Task) error
	DeleteTask(ctx context.Context, id int64) error
}

type MemoryStore interface {
	CreateMemory(ctx context.Context, mem *model.ConversationMemory) error
	GetMemory(ctx context.Context, conversationID int64, key string) (*model.ConversationMemory, error)
	ListMemories(ctx context.Context, conversationID int64) ([]*model.ConversationMemory, error)
	UpsertMemory(ctx context.Context, mem *model.ConversationMemory) error
	DeleteMemory(ctx context.Context, id int64) error
}

type ChangeRequestStore interface {
	CreateChangeRequest(ctx context.Context, cr *model.ChangeRequest) error
	ListChangeRequests(ctx context.Context, conversationID int64, status string) ([]*model.ChangeRequest, error)
	GetChangeRequest(ctx context.Context, id int64) (*model.ChangeRequest, error)
	ResolveChangeRequest(ctx context.Context, id int64, approverID int64, approved bool) error
}

type AuditStore interface {
	CreateAuditLog(ctx context.Context, log *model.AuditLog) error
	ListAuditLogs(ctx context.Context, entityID *int64, action string, since string, limit, offset int) ([]*model.AuditLog, int, error)
}
