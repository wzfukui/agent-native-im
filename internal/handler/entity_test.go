package handler_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gorillaWs "github.com/gorilla/websocket"
)

func TestCreateBot(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/entities", ptr(token), map[string]string{
		"name":         "test-agent",
		"display_name": "Test Agent",
	})
	assertStatus(t, resp, http.StatusCreated)

	data := parseOK(t, resp)

	// Should have bootstrap key with aimb_ prefix
	bootstrapKey, _ := data["bootstrap_key"].(string)
	if !strings.HasPrefix(bootstrapKey, "aimb_") {
		t.Fatalf("expected bootstrap key with aimb_ prefix, got %q", bootstrapKey)
	}

	// Should have markdown doc
	markdownDoc, _ := data["markdown_doc"].(string)
	if markdownDoc == "" {
		t.Fatal("expected non-empty markdown_doc")
	}
	if !strings.Contains(markdownDoc, bootstrapKey) {
		t.Fatal("markdown_doc should contain the bootstrap key")
	}
	if !strings.Contains(markdownDoc, "Agent 接入指南") {
		t.Fatal("markdown_doc should contain onboarding instructions")
	}

	// Entity should exist
	entity, _ := data["entity"].(map[string]interface{})
	if entity["entity_type"] != "bot" {
		t.Fatalf("expected entity_type=bot, got %v", entity["entity_type"])
	}
}

func TestBootstrapKeyRestrictions(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create bot and get bootstrap key
	resp := doJSON(t, "POST", "/api/v1/entities", ptr(token), map[string]string{
		"name": "restricted-agent",
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	bootstrapKey, _ := data["bootstrap_key"].(string)

	// Bootstrap key CAN access /me
	resp = doJSON(t, "GET", "/api/v1/me", ptr(bootstrapKey), nil)
	assertStatus(t, resp, http.StatusOK)

	// Bootstrap key CANNOT access /conversations
	resp = doJSON(t, "GET", "/api/v1/conversations", ptr(bootstrapKey), nil)
	assertStatus(t, resp, http.StatusForbidden)

	// Bootstrap key CANNOT send messages
	resp = doJSON(t, "POST", "/api/v1/messages/send", ptr(bootstrapKey), map[string]interface{}{
		"conversation_id": 1,
		"layers":          map[string]string{"summary": "hello"},
	})
	assertStatus(t, resp, http.StatusForbidden)
}

func TestApproveConnection(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create bot
	resp := doJSON(t, "POST", "/api/v1/entities", ptr(token), map[string]string{
		"name": "approve-agent",
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	bootstrapKey, _ := data["bootstrap_key"].(string)
	entity, _ := data["entity"].(map[string]interface{})
	entityID := entity["id"].(float64)

	// Approve connection
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/entities/%d/approve", int(entityID)), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)

	// Bootstrap key should no longer work
	resp = doJSON(t, "GET", "/api/v1/me", ptr(bootstrapKey), nil)
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestApproveConnectionWSPush(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create bot
	resp := doJSON(t, "POST", "/api/v1/entities", ptr(token), map[string]string{
		"name": "ws-approve-agent",
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	bootstrapKey, _ := data["bootstrap_key"].(string)
	entity, _ := data["entity"].(map[string]interface{})
	entityID := int(entity["id"].(float64))

	// Start a test server for WebSocket
	ts := newWSTestServer(t)
	defer ts.Close()

	// Connect WebSocket with bootstrap key
	wsURL := fmt.Sprintf("ws%s/api/v1/ws?token=%s", ts.URL[len("http"):], bootstrapKey)
	wsConn, _, err := gorillaWs.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	defer wsConn.Close()

	// Drain entity.config
	skipEntityConfig(t, wsConn)

	// Approve connection via HTTP
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/entities/%d/approve", entityID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)

	// Read WebSocket message — should receive connection.approved with permanent key
	wsConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var wsMsg map[string]interface{}
	if err := wsConn.ReadJSON(&wsMsg); err != nil {
		t.Fatalf("ws read: %v", err)
	}

	if wsMsg["type"] != "connection.approved" {
		t.Fatalf("expected type=connection.approved, got %v", wsMsg["type"])
	}

	wsData, _ := wsMsg["data"].(map[string]interface{})
	permanentKey, _ := wsData["api_key"].(string)
	if !strings.HasPrefix(permanentKey, "aim_") {
		t.Fatalf("expected permanent key with aim_ prefix, got %q", permanentKey)
	}

	// Permanent key should work for full auth
	resp = doJSON(t, "GET", "/api/v1/conversations", ptr(permanentKey), nil)
	assertStatus(t, resp, http.StatusOK)
}

func TestListEntities(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create two bots
	doJSON(t, "POST", "/api/v1/entities", ptr(token), map[string]string{"name": "bot-1"})
	doJSON(t, "POST", "/api/v1/entities", ptr(token), map[string]string{"name": "bot-2"})

	resp := doJSON(t, "GET", "/api/v1/entities", ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)

	result := parseResponse(t, resp)
	entities, ok := result["data"].([]interface{})
	if !ok || len(entities) < 2 {
		t.Fatalf("expected at least 2 entities, got %v", result["data"])
	}
}

func TestDeleteEntity(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/entities", ptr(token), map[string]string{"name": "delete-me"})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	entity, _ := data["entity"].(map[string]interface{})
	entityID := int(entity["id"].(float64))

	resp = doJSON(t, "DELETE", fmt.Sprintf("/api/v1/entities/%d", entityID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
}

func TestEntityStatus(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create bot
	resp := doJSON(t, "POST", "/api/v1/entities", ptr(token), map[string]string{"name": "status-agent"})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	entity, _ := data["entity"].(map[string]interface{})
	entityID := int(entity["id"].(float64))

	// Check status — should be offline
	resp = doJSON(t, "GET", fmt.Sprintf("/api/v1/entities/%d/status", entityID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
	statusData := parseOK(t, resp)
	if statusData["online"] != false {
		t.Fatalf("expected online=false, got %v", statusData["online"])
	}
}

func TestWebhookOwnership(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create webhook
	resp := doJSON(t, "POST", "/api/v1/webhooks", ptr(token), map[string]interface{}{
		"url": "http://example.com/hook",
	})
	assertStatus(t, resp, http.StatusCreated)
	whData := parseOK(t, resp)
	webhook, _ := whData["webhook"].(map[string]interface{})
	whID := int(webhook["id"].(float64))

	// Create a second user
	resp = doJSON(t, "POST", "/api/v1/admin/users", ptr(token), map[string]string{
		"username": "other-user",
		"password": "Otherpass1",
	})
	assertStatus(t, resp, http.StatusCreated)
	otherToken := login(t, "other-user", "Otherpass1")

	// Other user tries to delete admin's webhook — should fail
	resp = doJSON(t, "DELETE", fmt.Sprintf("/api/v1/webhooks/%d", whID), ptr(otherToken), nil)
	assertStatus(t, resp, http.StatusForbidden)

	// Owner can delete their own webhook
	resp = doJSON(t, "DELETE", fmt.Sprintf("/api/v1/webhooks/%d", whID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
}

func TestEntityListWithOnlineStatus(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create a bot
	resp := doJSON(t, "POST", "/api/v1/entities", ptr(token), map[string]string{"name": "online-bot"})
	assertStatus(t, resp, http.StatusCreated)

	// List entities — should include online field
	resp = doJSON(t, "GET", "/api/v1/entities", ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)

	result := parseResponse(t, resp)
	entities, ok := result["data"].([]interface{})
	if !ok || len(entities) < 1 {
		t.Fatalf("expected at least 1 entity, got %v", result["data"])
	}

	e0 := entities[0].(map[string]interface{})
	// Should have "online" field
	if _, exists := e0["online"]; !exists {
		t.Fatal("expected entity to have 'online' field")
	}
}

func TestUpdateEntityDisplayName(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create bot
	resp := doJSON(t, "POST", "/api/v1/entities", ptr(token), map[string]string{"name": "update-me"})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	entity, _ := data["entity"].(map[string]interface{})
	entityID := int(entity["id"].(float64))

	// Update display name
	resp = doJSON(t, "PUT", fmt.Sprintf("/api/v1/entities/%d", entityID), ptr(token), map[string]interface{}{
		"display_name": "New Display Name",
	})
	assertStatus(t, resp, http.StatusOK)

	updated := parseOK(t, resp)
	if updated["display_name"] != "New Display Name" {
		t.Fatalf("expected display_name='New Display Name', got %v", updated["display_name"])
	}
}

func TestUpdateEntityMetadata(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create bot
	resp := doJSON(t, "POST", "/api/v1/entities", ptr(token), map[string]string{"name": "meta-bot"})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	entity, _ := data["entity"].(map[string]interface{})
	entityID := int(entity["id"].(float64))

	// Set metadata
	resp = doJSON(t, "PUT", fmt.Sprintf("/api/v1/entities/%d", entityID), ptr(token), map[string]interface{}{
		"metadata": map[string]interface{}{
			"description":  "A test bot",
			"capabilities": []string{"chat", "search"},
		},
	})
	assertStatus(t, resp, http.StatusOK)

	updated := parseOK(t, resp)
	meta, _ := updated["metadata"].(map[string]interface{})
	if meta["description"] != "A test bot" {
		t.Fatalf("expected description='A test bot', got %v", meta["description"])
	}

	// Merge more metadata (existing keys should persist)
	resp = doJSON(t, "PUT", fmt.Sprintf("/api/v1/entities/%d", entityID), ptr(token), map[string]interface{}{
		"metadata": map[string]interface{}{
			"version": "1.0",
		},
	})
	assertStatus(t, resp, http.StatusOK)

	updated = parseOK(t, resp)
	meta, _ = updated["metadata"].(map[string]interface{})
	if meta["description"] != "A test bot" {
		t.Fatal("metadata merge should preserve existing keys")
	}
	if meta["version"] != "1.0" {
		t.Fatal("metadata merge should add new keys")
	}

	// Delete a metadata key by setting to null
	resp = doJSON(t, "PUT", fmt.Sprintf("/api/v1/entities/%d", entityID), ptr(token), map[string]interface{}{
		"metadata": map[string]interface{}{
			"version": nil,
		},
	})
	assertStatus(t, resp, http.StatusOK)

	updated = parseOK(t, resp)
	meta, _ = updated["metadata"].(map[string]interface{})
	if _, exists := meta["version"]; exists {
		t.Fatal("setting metadata key to null should delete it")
	}
	if meta["description"] != "A test bot" {
		t.Fatal("other metadata keys should be preserved")
	}
}

func TestUpdateEntityOwnershipCheck(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create bot
	resp := doJSON(t, "POST", "/api/v1/entities", ptr(token), map[string]string{"name": "owned-bot"})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	entity, _ := data["entity"].(map[string]interface{})
	entityID := int(entity["id"].(float64))

	// Create another user
	doJSON(t, "POST", "/api/v1/admin/users", ptr(token), map[string]string{
		"username": "other",
		"password": "Other123",
	})
	otherToken := login(t, "other", "Other123")

	// Other user tries to update — should fail
	resp = doJSON(t, "PUT", fmt.Sprintf("/api/v1/entities/%d", entityID), ptr(otherToken), map[string]interface{}{
		"display_name": "Hacked",
	})
	assertStatus(t, resp, http.StatusForbidden)
}

func TestReactivateEntity(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create bot
	resp := doJSON(t, "POST", "/api/v1/entities", ptr(token), map[string]string{"name": "reactivate-me"})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	entity, _ := data["entity"].(map[string]interface{})
	entityID := int(entity["id"].(float64))

	// Soft-delete (disable)
	resp = doJSON(t, "DELETE", fmt.Sprintf("/api/v1/entities/%d", entityID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)

	// Verify disabled — list entities should include it with status=disabled
	resp = doJSON(t, "GET", "/api/v1/entities", ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
	result := parseResponse(t, resp)
	entities, _ := result["data"].([]interface{})
	found := false
	for _, e := range entities {
		em := e.(map[string]interface{})
		if int(em["id"].(float64)) == entityID {
			if em["status"] != "disabled" {
				t.Fatalf("expected status=disabled, got %v", em["status"])
			}
			found = true
		}
	}
	if !found {
		t.Fatal("disabled entity should still appear in list")
	}

	// Reactivate
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/entities/%d/reactivate", entityID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)

	reactivated := parseOK(t, resp)
	if reactivated["status"] != "active" {
		t.Fatalf("expected status=active after reactivation, got %v", reactivated["status"])
	}
}

func TestReactivateActiveEntity(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create bot (status=active by default)
	resp := doJSON(t, "POST", "/api/v1/entities", ptr(token), map[string]string{"name": "already-active"})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	entity, _ := data["entity"].(map[string]interface{})
	entityID := int(entity["id"].(float64))

	// Try to reactivate an active entity — should fail
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/entities/%d/reactivate", entityID), ptr(token), nil)
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestListEntitiesIncludesDisabled(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create two bots
	resp := doJSON(t, "POST", "/api/v1/entities", ptr(token), map[string]string{"name": "active-bot"})
	assertStatus(t, resp, http.StatusCreated)

	resp = doJSON(t, "POST", "/api/v1/entities", ptr(token), map[string]string{"name": "will-disable"})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	entity, _ := data["entity"].(map[string]interface{})
	disabledID := int(entity["id"].(float64))

	// Disable one bot
	resp = doJSON(t, "DELETE", fmt.Sprintf("/api/v1/entities/%d", disabledID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)

	// List should return both
	resp = doJSON(t, "GET", "/api/v1/entities", ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
	result := parseResponse(t, resp)
	entities, _ := result["data"].([]interface{})

	if len(entities) < 2 {
		t.Fatalf("expected at least 2 entities (active + disabled), got %d", len(entities))
	}

	// Active entities should come first
	first := entities[0].(map[string]interface{})
	if first["status"] == "disabled" {
		t.Fatal("active entities should be sorted before disabled ones")
	}
}

// newWSTestServer creates an httptest.Server wired to the same router for WebSocket testing.
func newWSTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(testRouter)
}
