package handler_test

import (
	"fmt"
	"net/http"
	"testing"
)

func sendMessage(t *testing.T, token string, convID int, text string) int {
	t.Helper()
	resp := doJSON(t, "POST", "/api/v1/messages/send", ptr(token), map[string]interface{}{
		"conversation_id": convID,
		"layers":          map[string]string{"summary": text},
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	return int(data["id"].(float64))
}

func TestToggleReaction(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)
	convID := setupConversation(t, token)
	msgID := sendMessage(t, token, convID, "React to me")

	// Add a reaction
	resp := doJSON(t, "POST", fmt.Sprintf("/api/v1/messages/%d/reactions", msgID), ptr(token), map[string]string{
		"emoji": "👍",
	})
	assertStatus(t, resp, http.StatusOK)

	data := parseOK(t, resp)
	reactions, ok := data["reactions"].([]interface{})
	if !ok || len(reactions) != 1 {
		t.Fatalf("expected 1 reaction, got %v", data["reactions"])
	}

	r0 := reactions[0].(map[string]interface{})
	if r0["emoji"] != "👍" {
		t.Fatalf("expected emoji=👍, got %v", r0["emoji"])
	}
	if r0["count"].(float64) != 1 {
		t.Fatalf("expected count=1, got %v", r0["count"])
	}

	// Toggle off (same emoji again)
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/messages/%d/reactions", msgID), ptr(token), map[string]string{
		"emoji": "👍",
	})
	assertStatus(t, resp, http.StatusOK)

	data = parseOK(t, resp)
	reactions, ok = data["reactions"].([]interface{})
	if !ok || len(reactions) != 0 {
		t.Fatalf("expected 0 reactions after toggle off, got %v", data["reactions"])
	}
}

func TestMultipleReactions(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)
	convID := setupConversation(t, token)
	msgID := sendMessage(t, token, convID, "Multi react")

	// Add two different emojis
	resp := doJSON(t, "POST", fmt.Sprintf("/api/v1/messages/%d/reactions", msgID), ptr(token), map[string]string{
		"emoji": "👍",
	})
	assertStatus(t, resp, http.StatusOK)

	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/messages/%d/reactions", msgID), ptr(token), map[string]string{
		"emoji": "❤️",
	})
	assertStatus(t, resp, http.StatusOK)

	data := parseOK(t, resp)
	reactions, ok := data["reactions"].([]interface{})
	if !ok || len(reactions) != 2 {
		t.Fatalf("expected 2 reactions, got %v", data["reactions"])
	}
}

func TestReactionNotParticipant(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)
	convID := setupConversation(t, token)
	msgID := sendMessage(t, token, convID, "Restricted")

	// Create another user
	doJSON(t, "POST", "/api/v1/admin/users", ptr(token), map[string]string{
		"username": "outsider",
		"password": "outsider123",
	})
	outsiderToken := login(t, "outsider", "outsider123")

	// Non-participant tries to react — should be forbidden
	resp := doJSON(t, "POST", fmt.Sprintf("/api/v1/messages/%d/reactions", msgID), ptr(outsiderToken), map[string]string{
		"emoji": "👍",
	})
	assertStatus(t, resp, http.StatusForbidden)
}

func TestReactionsInListMessages(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)
	convID := setupConversation(t, token)
	msgID := sendMessage(t, token, convID, "With reactions")

	// Add a reaction
	resp := doJSON(t, "POST", fmt.Sprintf("/api/v1/messages/%d/reactions", msgID), ptr(token), map[string]string{
		"emoji": "🎉",
	})
	assertStatus(t, resp, http.StatusOK)

	// List messages — should include reactions
	resp = doJSON(t, "GET", fmt.Sprintf("/api/v1/conversations/%d/messages", convID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)

	data := parseOK(t, resp)
	msgs, ok := data["messages"].([]interface{})
	if !ok || len(msgs) == 0 {
		t.Fatal("expected at least 1 message")
	}

	msg := msgs[0].(map[string]interface{})
	reactions, ok := msg["reactions"].([]interface{})
	if !ok || len(reactions) != 1 {
		t.Fatalf("expected message to have 1 reaction, got %v", msg["reactions"])
	}

	r0 := reactions[0].(map[string]interface{})
	if r0["emoji"] != "🎉" {
		t.Fatalf("expected emoji=🎉, got %v", r0["emoji"])
	}
}
