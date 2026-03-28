package handler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
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

	// Should have permanent API key with aim_ prefix (no bootstrap flow)
	apiKey, _ := data["api_key"].(string)
	if !strings.HasPrefix(apiKey, "aim_") {
		t.Fatalf("expected API key with aim_ prefix, got %q", apiKey)
	}

	// Should have markdown doc
	markdownDoc, _ := data["markdown_doc"].(string)
	if markdownDoc == "" {
		t.Fatal("expected non-empty markdown_doc")
	}
	if !strings.Contains(markdownDoc, apiKey) {
		t.Fatal("markdown_doc should contain the API key")
	}
	if !strings.Contains(markdownDoc, "OpenClaw Access Pack") {
		t.Fatal("markdown_doc should default to the OpenClaw access pack")
	}
	if strings.Contains(markdownDoc, "Bootstrap") {
		t.Fatal("markdown_doc should not mention Bootstrap")
	}
	if strings.Contains(markdownDoc, "agent-native-im-sdk-python") {
		t.Fatal("markdown_doc should not direct OpenClaw users to the Python SDK")
	}
	if !strings.Contains(markdownDoc, "openclaw plugins install ani-openclaw-plugin") {
		t.Fatal("markdown_doc should contain the npm install path")
	}
	if strings.Contains(markdownDoc, "Entity ID") {
		t.Fatal("markdown_doc should not include obsolete entity id guidance")
	}
	if strings.Contains(markdownDoc, "group:web") {
		t.Fatal("markdown_doc should not include unrelated web tool permissions")
	}

	// Entity should exist
	entity, _ := data["entity"].(map[string]interface{})
	if entity["entity_type"] != "bot" {
		t.Fatalf("expected entity_type=bot, got %v", entity["entity_type"])
	}
	if entity["name"] != "bot_test_agent" {
		t.Fatalf("expected persisted bot name=bot_test_agent, got %v", entity["name"])
	}
	if entity["bot_id"] != "bot_test_agent" {
		t.Fatalf("expected bot_id=bot_test_agent, got %v", entity["bot_id"])
	}
	publicID, _ := entity["public_id"].(string)
	if _, err := uuid.Parse(publicID); err != nil {
		t.Fatalf("expected valid public_id UUID, got %q", publicID)
	}

	// Permanent key should immediately have full API access
	resp = doJSON(t, "GET", "/api/v1/conversations", ptr(apiKey), nil)
	assertStatus(t, resp, http.StatusOK)
}

func TestCreateBotRequiresExplicitBotID(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	reqBody := map[string]string{
		"name": "missing-bot-id",
	}
	b, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/entities", strings.NewReader(string(b)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestCreateBotRejectsInvalidBotIDFormat(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/entities", ptr(token), map[string]string{
		"name":   "bad-bot",
		"bot_id": "supportbot",
	})
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestCreatedKeyHasFullAccess(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create bot — should get a permanent key with full access
	resp := doJSON(t, "POST", "/api/v1/entities", ptr(token), map[string]string{
		"name": "full-access-agent",
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	apiKey, _ := data["api_key"].(string)

	// Permanent key CAN access /me
	resp = doJSON(t, "GET", "/api/v1/me", ptr(apiKey), nil)
	assertStatus(t, resp, http.StatusOK)

	// Permanent key CAN access /conversations
	resp = doJSON(t, "GET", "/api/v1/conversations", ptr(apiKey), nil)
	assertStatus(t, resp, http.StatusOK)
}

func TestApproveConnectionBackwardCompat(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create bot — now gets permanent key
	resp := doJSON(t, "POST", "/api/v1/entities", ptr(token), map[string]string{
		"name": "approve-agent",
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	apiKey, _ := data["api_key"].(string)
	entity, _ := data["entity"].(map[string]interface{})
	entityID := entity["id"].(float64)

	// Approve endpoint still works (backward compat) — no bootstrap to delete, but still succeeds
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/entities/%d/approve", int(entityID)), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)

	// Original permanent key should still work (it was never a bootstrap key)
	resp = doJSON(t, "GET", "/api/v1/me", ptr(apiKey), nil)
	assertStatus(t, resp, http.StatusOK)
}

func TestWSConnectWithPermanentKey(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create bot — gets permanent key directly
	resp := doJSON(t, "POST", "/api/v1/entities", ptr(token), map[string]string{
		"name": "ws-agent",
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	apiKey, _ := data["api_key"].(string)

	// Start a test server for WebSocket
	ts := newWSTestServer(t)
	defer ts.Close()

	// Connect WebSocket with permanent key — should work immediately
	wsURL := fmt.Sprintf("ws%s/api/v1/ws", ts.URL[len("http"):])
	wsConn, _, err := gorillaWs.DefaultDialer.Dial(wsURL, http.Header{"Authorization": []string{"Bearer " + apiKey}})
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	defer wsConn.Close()

	// Drain entity.config
	skipEntityConfig(t, wsConn)

	// Permanent key should work for full auth
	resp = doJSON(t, "GET", "/api/v1/conversations", ptr(apiKey), nil)
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

func TestEntityStatusOwnership(t *testing.T) {
	truncateAll(t)
	adminToken := seedAdmin(t)

	// Create bot by admin
	resp := doJSON(t, "POST", "/api/v1/entities", ptr(adminToken), map[string]string{"name": "owned-status-agent"})
	assertStatus(t, resp, http.StatusCreated)
	entityID := int(parseOK(t, resp)["entity"].(map[string]interface{})["id"].(float64))

	// Create other user
	resp = doJSON(t, "POST", "/api/v1/admin/users", ptr(adminToken), map[string]string{
		"username": "status-other-user",
		"password": "Otherpass1",
	})
	assertStatus(t, resp, http.StatusCreated)
	otherToken := login(t, "status-other-user", "Otherpass1")

	// Non-owner should be forbidden
	resp = doJSON(t, "GET", fmt.Sprintf("/api/v1/entities/%d/status", entityID), ptr(otherToken), nil)
	assertStatus(t, resp, http.StatusForbidden)
}

func TestEntitySelfCheckAndDiagnostics(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create bot
	resp := doJSON(t, "POST", "/api/v1/entities", ptr(token), map[string]string{"name": "diag-agent"})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	entity := data["entity"].(map[string]interface{})
	entityID := int(entity["id"].(float64))

	// Self-check should be accessible and include readiness fields
	resp = doJSON(t, "GET", fmt.Sprintf("/api/v1/entities/%d/self-check", entityID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
	selfCheck := parseOK(t, resp)
	if _, ok := selfCheck["ready"].(bool); !ok {
		t.Fatalf("expected ready bool in self-check, got %T", selfCheck["ready"])
	}
	if _, ok := selfCheck["recommendation"].([]interface{}); !ok {
		t.Fatalf("expected recommendation list in self-check, got %T", selfCheck["recommendation"])
	}

	// Diagnostics should be accessible and include connection counters
	resp = doJSON(t, "GET", fmt.Sprintf("/api/v1/entities/%d/diagnostics", entityID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
	diag := parseOK(t, resp)
	if _, ok := diag["connections"].(float64); !ok {
		t.Fatalf("expected connections number in diagnostics, got %T", diag["connections"])
	}
	if _, ok := diag["hub"].(map[string]interface{}); !ok {
		t.Fatalf("expected hub object in diagnostics, got %T", diag["hub"])
	}
	if _, ok := diag["disconnect_count"].(float64); !ok {
		t.Fatalf("expected disconnect_count number in diagnostics, got %T", diag["disconnect_count"])
	}
}

func TestRegenerateEntityToken(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/entities", ptr(token), map[string]string{"name": "regen-agent"})
	assertStatus(t, resp, http.StatusCreated)
	entityID := int(parseOK(t, resp)["entity"].(map[string]interface{})["id"].(float64))

	// First rotation creates a permanent key
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/entities/%d/regenerate-token", entityID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
	firstData := parseOK(t, resp)
	firstKey, _ := firstData["api_key"].(string)
	if !strings.HasPrefix(firstKey, "aim_") {
		t.Fatalf("expected permanent key prefix aim_, got %q", firstKey)
	}

	// First key can access full-auth endpoint
	resp = doJSON(t, "GET", "/api/v1/conversations", ptr(firstKey), nil)
	assertStatus(t, resp, http.StatusOK)

	// Second rotation invalidates first key
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/entities/%d/regenerate-token", entityID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
	secondData := parseOK(t, resp)
	secondKey, _ := secondData["api_key"].(string)
	if !strings.HasPrefix(secondKey, "aim_") {
		t.Fatalf("expected permanent key prefix aim_, got %q", secondKey)
	}
	if secondKey == firstKey {
		t.Fatal("expected regenerated key to differ from previous key")
	}

	resp = doJSON(t, "GET", "/api/v1/conversations", ptr(firstKey), nil)
	assertStatus(t, resp, http.StatusUnauthorized)
	resp = doJSON(t, "GET", "/api/v1/conversations", ptr(secondKey), nil)
	assertStatus(t, resp, http.StatusOK)
}

func TestRegenerateEntityTokenOwnership(t *testing.T) {
	truncateAll(t)
	adminToken := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/entities", ptr(adminToken), map[string]string{"name": "regen-owned-agent"})
	assertStatus(t, resp, http.StatusCreated)
	entityID := int(parseOK(t, resp)["entity"].(map[string]interface{})["id"].(float64))

	resp = doJSON(t, "POST", "/api/v1/admin/users", ptr(adminToken), map[string]string{
		"username": "regen-other-user",
		"password": "Otherpass1",
	})
	assertStatus(t, resp, http.StatusCreated)
	otherToken := login(t, "regen-other-user", "Otherpass1")

	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/entities/%d/regenerate-token", entityID), ptr(otherToken), nil)
	assertStatus(t, resp, http.StatusForbidden)
}

func TestUpdateEntityNormalizesStableAvatarRoutes(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/entities", ptr(token), map[string]string{"name": "avatar-normalize-bot"})
	assertStatus(t, resp, http.StatusCreated)
	entityID := int(parseOK(t, resp)["entity"].(map[string]interface{})["id"].(float64))

	resp = doJSON(t, "PUT", fmt.Sprintf("/api/v1/entities/%d", entityID), ptr(token), map[string]interface{}{
		"avatar_url": "/avatar-files/bot.png?v=1",
	})
	assertStatus(t, resp, http.StatusOK)
	data := parseOK(t, resp)
	if data["avatar_url"] != "/files/bot.png" {
		t.Fatalf("expected normalized entity avatar_url=/files/bot.png, got %v", data["avatar_url"])
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
