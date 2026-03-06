package handler_test

import (
	"fmt"
	"net/http"
	"testing"
)

func TestUpsertMemory(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title": "Memory Test",
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	convID := int(data["id"].(float64))

	// Create memory
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/memories", convID), ptr(token), map[string]interface{}{
		"key":     "project_goal",
		"content": "Build an IM system",
	})
	assertStatus(t, resp, http.StatusOK)
	data = parseOK(t, resp)
	if data["key"] != "project_goal" {
		t.Fatalf("expected key 'project_goal', got %v", data["key"])
	}
	if data["content"] != "Build an IM system" {
		t.Fatalf("expected content 'Build an IM system', got %v", data["content"])
	}
}

func TestUpsertMemoryUpdate(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title": "Memory Upsert Test",
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	convID := int(data["id"].(float64))

	// Create
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/memories", convID), ptr(token), map[string]interface{}{
		"key":     "status",
		"content": "planning",
	})
	assertStatus(t, resp, http.StatusOK)

	// Upsert same key with new content
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/memories", convID), ptr(token), map[string]interface{}{
		"key":     "status",
		"content": "in progress",
	})
	assertStatus(t, resp, http.StatusOK)
	data = parseOK(t, resp)
	if data["content"] != "in progress" {
		t.Fatalf("expected content 'in progress', got %v", data["content"])
	}

	// List should have only 1 entry for key "status"
	resp = doJSON(t, "GET", fmt.Sprintf("/api/v1/conversations/%d/memories", convID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
	result := parseOK(t, resp)
	memories, ok := result["memories"].([]interface{})
	if !ok {
		t.Fatalf("expected memories array, got %v", result["memories"])
	}
	if len(memories) != 1 {
		t.Fatalf("expected 1 memory, got %d", len(memories))
	}
}

func TestListMemories(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title": "Memory List Test",
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	convID := int(data["id"].(float64))

	// Create multiple memories
	doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/memories", convID), ptr(token), map[string]interface{}{
		"key":     "goal",
		"content": "Ship v1",
	})
	doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/memories", convID), ptr(token), map[string]interface{}{
		"key":     "deadline",
		"content": "March 2026",
	})

	// List
	resp = doJSON(t, "GET", fmt.Sprintf("/api/v1/conversations/%d/memories", convID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
	result := parseOK(t, resp)
	memories := result["memories"].([]interface{})
	if len(memories) != 2 {
		t.Fatalf("expected 2 memories, got %d", len(memories))
	}
}

func TestDeleteMemory(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title": "Memory Delete Test",
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	convID := int(data["id"].(float64))

	// Create memory
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/memories", convID), ptr(token), map[string]interface{}{
		"key":     "temp",
		"content": "to be deleted",
	})
	assertStatus(t, resp, http.StatusOK)
	data = parseOK(t, resp)
	memID := int(data["id"].(float64))

	// Delete
	resp = doJSON(t, "DELETE", fmt.Sprintf("/api/v1/conversations/%d/memories/%d", convID, memID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)

	// List should be empty
	resp = doJSON(t, "GET", fmt.Sprintf("/api/v1/conversations/%d/memories", convID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
	result := parseOK(t, resp)
	if memories, ok := result["memories"].([]interface{}); ok && len(memories) != 0 {
		t.Fatalf("expected 0 memories after delete, got %d", len(memories))
	}
}

func TestMemoryNotParticipant(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	doJSON(t, "POST", "/api/v1/admin/users", ptr(token), map[string]string{
		"username": "outsider",
		"password": "Outsider123",
	})
	outsiderToken := login(t, "outsider", "Outsider123")

	resp := doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title": "Private Conv",
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	convID := int(data["id"].(float64))

	// Outsider tries to list memories
	resp = doJSON(t, "GET", fmt.Sprintf("/api/v1/conversations/%d/memories", convID), ptr(outsiderToken), nil)
	assertStatus(t, resp, http.StatusForbidden)

	// Outsider tries to create memory
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/memories", convID), ptr(outsiderToken), map[string]interface{}{
		"key":     "hack",
		"content": "bad data",
	})
	assertStatus(t, resp, http.StatusForbidden)
}
