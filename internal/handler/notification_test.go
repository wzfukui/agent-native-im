package handler

import (
	"encoding/json"
	"testing"

	"github.com/wzfukui/agent-native-im/internal/model"
)

func TestNotificationPushPathUsesConversationWhenPresent(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{"conversation_id": 42})
	notification := &model.Notification{RecipientEntityID: 7, Kind: "conversation.change_request", Data: payload}
	if got := notificationPushPath(notification); got != "/chat/42" {
		t.Fatalf("expected conversation path, got %q", got)
	}
}

func TestNotificationPushPathPrefersConversationPublicID(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{
		"conversation_id":        42,
		"conversation_public_id": "2dca4d32-79bc-45b0-b6df-e5038bb6ad70",
	})
	notification := &model.Notification{RecipientEntityID: 7, Kind: "conversation.change_request", Data: payload}
	if got := notificationPushPath(notification); got != "/chat/public/2dca4d32-79bc-45b0-b6df-e5038bb6ad70" {
		t.Fatalf("expected public conversation path, got %q", got)
	}
}
func TestNotificationPushPathRoutesFriendNotificationsToFriends(t *testing.T) {
	notification := &model.Notification{RecipientEntityID: 7, Kind: "friend.request.received"}
	if got := notificationPushPath(notification); got != "/friends" {
		t.Fatalf("expected /friends path, got %q", got)
	}
}

func TestNotificationPushPathFallsBackToInboxScope(t *testing.T) {
	notification := &model.Notification{RecipientEntityID: 19, Kind: "public.bot_session_created"}
	if got := notificationPushPath(notification); got != "/inbox?scope=19" {
		t.Fatalf("expected scoped inbox path, got %q", got)
	}
}
