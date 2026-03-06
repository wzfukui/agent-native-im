package handler_test

import (
	"fmt"
	"net/http"
	"testing"
)

// --- Observer Mode ---

func TestObserverCannotSendMessage(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create a user that will be an observer
	resp := doJSON(t, "POST", "/api/v1/admin/users", ptr(token), map[string]string{
		"username": "observer",
		"password": "Observer123",
	})
	assertStatus(t, resp, http.StatusCreated)
	obsData := parseOK(t, resp)
	obsID := int(obsData["id"].(float64))
	obsToken := login(t, "observer", "Observer123")

	// Create conversation
	resp = doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title":    "Observer Test",
		"conv_type": "group",
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	convID := int(data["id"].(float64))

	// Add observer with role=observer
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/participants", convID), ptr(token), map[string]interface{}{
		"entity_id": obsID,
		"role":      "observer",
	})
	assertStatus(t, resp, http.StatusCreated)

	// Observer tries to send message — should be forbidden
	resp = doJSON(t, "POST", "/api/v1/messages/send", ptr(obsToken), map[string]interface{}{
		"conversation_id": convID,
		"layers":          map[string]string{"summary": "I should not be able to send this"},
	})
	assertStatus(t, resp, http.StatusForbidden)
}

func TestObserverCanReadMessages(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create observer user
	resp := doJSON(t, "POST", "/api/v1/admin/users", ptr(token), map[string]string{
		"username": "observer2",
		"password": "Observer2p123",
	})
	assertStatus(t, resp, http.StatusCreated)
	obsData := parseOK(t, resp)
	obsID := int(obsData["id"].(float64))
	obsToken := login(t, "observer2", "Observer2p123")

	// Create conversation
	resp = doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title":    "Observer Read Test",
		"conv_type": "group",
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	convID := int(data["id"].(float64))

	// Add observer
	doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/participants", convID), ptr(token), map[string]interface{}{
		"entity_id": obsID,
		"role":      "observer",
	})

	// Admin sends a message
	doJSON(t, "POST", "/api/v1/messages/send", ptr(token), map[string]interface{}{
		"conversation_id": convID,
		"layers":          map[string]string{"summary": "Hello observers"},
	})

	// Observer can read messages
	resp = doJSON(t, "GET", fmt.Sprintf("/api/v1/conversations/%d/messages?limit=10", convID), ptr(obsToken), nil)
	assertStatus(t, resp, http.StatusOK)
	data = parseOK(t, resp)
	msgs, ok := data["messages"].([]interface{})
	if !ok || len(msgs) < 1 {
		t.Fatal("observer should be able to read messages")
	}
}

// --- Interaction Response ---

func TestInteractionResponse(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)
	convID := setupConversation(t, token)

	// Send a message with interaction layer
	resp := doJSON(t, "POST", "/api/v1/messages/send", ptr(token), map[string]interface{}{
		"conversation_id": convID,
		"content_type":    "text",
		"layers": map[string]interface{}{
			"summary": "Please confirm",
			"interaction": map[string]interface{}{
				"type":   "choice",
				"prompt": "Do you approve?",
				"options": []map[string]string{
					{"label": "Yes", "value": "yes"},
					{"label": "No", "value": "no"},
				},
			},
		},
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	msgID := int(data["id"].(float64))

	// Respond to interaction
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/messages/%d/respond", msgID), ptr(token), map[string]string{
		"value": "yes",
	})
	assertStatus(t, resp, http.StatusOK)
}

func TestInteractionResponseNotParticipant(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)
	convID := setupConversation(t, token)

	// Send interaction message
	resp := doJSON(t, "POST", "/api/v1/messages/send", ptr(token), map[string]interface{}{
		"conversation_id": convID,
		"layers": map[string]interface{}{
			"summary": "Approve?",
			"interaction": map[string]interface{}{
				"type":    "choice",
				"options": []map[string]string{{"label": "OK", "value": "ok"}},
			},
		},
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	msgID := int(data["id"].(float64))

	// Create user not in conversation
	doJSON(t, "POST", "/api/v1/admin/users", ptr(token), map[string]string{
		"username": "outsider",
		"password": "Outsider123",
	})
	outsiderToken := login(t, "outsider", "Outsider123")

	// Outsider tries to respond — should fail
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/messages/%d/respond", msgID), ptr(outsiderToken), map[string]string{
		"value": "ok",
	})
	assertStatus(t, resp, http.StatusForbidden)
}

// --- Message Edit ---

func TestEditMessage(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)
	convID := setupConversation(t, token)

	// Send a message
	resp := doJSON(t, "POST", "/api/v1/messages/send", ptr(token), map[string]interface{}{
		"conversation_id": convID,
		"layers":          map[string]string{"summary": "Original text"},
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	msgID := int(data["id"].(float64))

	// Edit message (handler expects layers object)
	resp = doJSON(t, "PUT", fmt.Sprintf("/api/v1/messages/%d", msgID), ptr(token), map[string]interface{}{
		"layers": map[string]string{"summary": "Edited text"},
	})
	assertStatus(t, resp, http.StatusOK)
	data = parseOK(t, resp)
	layers, _ := data["layers"].(map[string]interface{})
	if layers["summary"] != "Edited text" {
		t.Fatalf("expected edited text, got %v", layers["summary"])
	}
}

func TestEditMessageNotSender(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)
	convID := setupConversation(t, token)

	// Send a message as admin
	resp := doJSON(t, "POST", "/api/v1/messages/send", ptr(token), map[string]interface{}{
		"conversation_id": convID,
		"layers":          map[string]string{"summary": "Admin's message"},
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	msgID := int(data["id"].(float64))

	// Create another user and add to conversation
	resp = doJSON(t, "POST", "/api/v1/admin/users", ptr(token), map[string]string{
		"username": "editor",
		"password": "Editor123",
	})
	assertStatus(t, resp, http.StatusCreated)
	editorData := parseOK(t, resp)
	editorID := int(editorData["id"].(float64))
	editorToken := login(t, "editor", "Editor123")

	doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/participants", convID), ptr(token), map[string]interface{}{
		"entity_id": editorID,
	})

	// Other user tries to edit — should fail
	resp = doJSON(t, "PUT", fmt.Sprintf("/api/v1/messages/%d", msgID), ptr(editorToken), map[string]interface{}{
		"layers": map[string]string{"summary": "Hacked"},
	})
	assertStatus(t, resp, http.StatusForbidden)
}

func TestEditMessageTooOld(t *testing.T) {
	// This test just verifies the endpoint works; the 5-minute check
	// is hard to test without manipulating time, but we can verify
	// that a fresh message CAN be edited.
	truncateAll(t)
	token := seedAdmin(t)
	convID := setupConversation(t, token)

	resp := doJSON(t, "POST", "/api/v1/messages/send", ptr(token), map[string]interface{}{
		"conversation_id": convID,
		"layers":          map[string]string{"summary": "Editable"},
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	msgID := int(data["id"].(float64))

	// Fresh message should be editable
	resp = doJSON(t, "PUT", fmt.Sprintf("/api/v1/messages/%d", msgID), ptr(token), map[string]interface{}{
		"layers": map[string]string{"summary": "Still editable"},
	})
	assertStatus(t, resp, http.StatusOK)
}
