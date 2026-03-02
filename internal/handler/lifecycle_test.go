package handler_test

import (
	"fmt"
	"net/http"
	"testing"
)

// --- Conversation Description ---

func TestUpdateConversationDescription(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create conversation
	resp := doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title": "Desc Test",
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	convID := int(data["id"].(float64))

	// Update description
	resp = doJSON(t, "PUT", fmt.Sprintf("/api/v1/conversations/%d", convID), ptr(token), map[string]interface{}{
		"description": "A test conversation for description",
	})
	assertStatus(t, resp, http.StatusOK)
	data = parseOK(t, resp)
	if data["description"] != "A test conversation for description" {
		t.Fatalf("expected description updated, got %v", data["description"])
	}

	// Verify via GET
	resp = doJSON(t, "GET", fmt.Sprintf("/api/v1/conversations/%d", convID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
	data = parseOK(t, resp)
	if data["description"] != "A test conversation for description" {
		t.Fatalf("expected description persisted, got %v", data["description"])
	}
}

func TestUpdateConversationTitleAndDescription(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title": "Original",
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	convID := int(data["id"].(float64))

	// Update both title and description
	resp = doJSON(t, "PUT", fmt.Sprintf("/api/v1/conversations/%d", convID), ptr(token), map[string]interface{}{
		"title":       "Updated Title",
		"description": "Updated Desc",
	})
	assertStatus(t, resp, http.StatusOK)
	data = parseOK(t, resp)
	if data["title"] != "Updated Title" {
		t.Fatalf("expected title updated, got %v", data["title"])
	}
	if data["description"] != "Updated Desc" {
		t.Fatalf("expected description updated, got %v", data["description"])
	}
}

// --- Leave Conversation ---

func TestLeaveConversation(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create a second user
	resp := doJSON(t, "POST", "/api/v1/admin/users", ptr(token), map[string]string{
		"username": "leaver",
		"password": "leaver123",
	})
	assertStatus(t, resp, http.StatusCreated)
	leaverData := parseOK(t, resp)
	leaverID := int(leaverData["id"].(float64))
	leaverToken := login(t, "leaver", "leaver123")

	// Create group with leaver as participant
	resp = doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title":           "Leave Test",
		"conv_type":       "group",
		"participant_ids": []float64{float64(leaverID)},
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	convID := int(data["id"].(float64))

	// Leaver leaves the conversation
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/leave", convID), ptr(leaverToken), nil)
	assertStatus(t, resp, http.StatusOK)

	// Leaver should no longer see this conversation
	resp = doJSON(t, "GET", "/api/v1/conversations", ptr(leaverToken), nil)
	assertStatus(t, resp, http.StatusOK)
	result := parseResponse(t, resp)
	convs, ok := result["data"].([]interface{})
	if !ok {
		t.Fatal("expected data array")
	}
	for _, c := range convs {
		conv := c.(map[string]interface{})
		if int(conv["id"].(float64)) == convID {
			t.Fatal("leaver should not see the conversation after leaving")
		}
	}
}

func TestLeaveConversationNotParticipant(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create user not in the conversation
	doJSON(t, "POST", "/api/v1/admin/users", ptr(token), map[string]string{
		"username": "outsider",
		"password": "outsider123",
	})
	outsiderToken := login(t, "outsider", "outsider123")

	// Create conversation (only admin)
	resp := doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title": "No Leave",
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	convID := int(data["id"].(float64))

	// Outsider tries to leave — should fail
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/leave", convID), ptr(outsiderToken), nil)
	assertStatus(t, resp, http.StatusForbidden)
}

// --- Archive / Unarchive ---

func TestArchiveUnarchiveConversation(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create conversation
	resp := doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title": "Archive Test",
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	convID := int(data["id"].(float64))

	// Archive
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/archive", convID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)

	// Archived conversation should not appear in default list
	resp = doJSON(t, "GET", "/api/v1/conversations", ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
	result := parseResponse(t, resp)
	convs, _ := result["data"].([]interface{})
	for _, c := range convs {
		conv := c.(map[string]interface{})
		if int(conv["id"].(float64)) == convID {
			t.Fatal("archived conversation should not appear in list")
		}
	}

	// Unarchive
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/unarchive", convID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)

	// Should reappear
	resp = doJSON(t, "GET", "/api/v1/conversations", ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
	result = parseResponse(t, resp)
	convs, _ = result["data"].([]interface{})
	found := false
	for _, c := range convs {
		conv := c.(map[string]interface{})
		if int(conv["id"].(float64)) == convID {
			found = true
		}
	}
	if !found {
		t.Fatal("unarchived conversation should appear in list")
	}
}

func TestArchiveNotParticipant(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	doJSON(t, "POST", "/api/v1/admin/users", ptr(token), map[string]string{
		"username": "archiver",
		"password": "archiver123",
	})
	archiverToken := login(t, "archiver", "archiver123")

	resp := doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title": "Not Mine",
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	convID := int(data["id"].(float64))

	// Non-participant tries to archive — should fail
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/archive", convID), ptr(archiverToken), nil)
	assertStatus(t, resp, http.StatusForbidden)
}
