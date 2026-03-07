package handler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

// --- Capability Search Tests ---

func TestSearchEntitiesByCapabilitySkills(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create bot with capabilities.skills
	resp := doJSON(t, "POST", "/api/v1/entities", ptr(token), map[string]interface{}{
		"name": "skill-bot",
		"metadata": map[string]interface{}{
			"capabilities": map[string]interface{}{
				"skills": []string{"code_review", "unit_testing"},
			},
		},
	})
	assertStatus(t, resp, http.StatusCreated)

	// Search by skill — should find it
	resp = doJSON(t, "GET", "/api/v1/entities/search?capability=code_review", ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)

	result := parseResponse(t, resp)
	entities, ok := result["data"].([]interface{})
	if !ok || len(entities) < 1 {
		t.Fatalf("expected at least 1 entity for code_review, got %v", result["data"])
	}
	e0 := entities[0].(map[string]interface{})
	if e0["name"] != "skill-bot" {
		t.Fatalf("expected name=skill-bot, got %v", e0["name"])
	}
	// Should include online field
	if _, exists := e0["online"]; !exists {
		t.Fatal("expected search results to include online field")
	}
}

func TestSearchEntitiesByCapabilityTags(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create bot with tags (backward compat)
	resp := doJSON(t, "POST", "/api/v1/entities", ptr(token), map[string]interface{}{
		"name": "tag-bot",
		"metadata": map[string]interface{}{
			"tags": []string{"billing", "support"},
		},
	})
	assertStatus(t, resp, http.StatusCreated)

	// Search by tag
	resp = doJSON(t, "GET", "/api/v1/entities/search?capability=billing", ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)

	result := parseResponse(t, resp)
	entities, ok := result["data"].([]interface{})
	if !ok || len(entities) < 1 {
		t.Fatalf("expected at least 1 entity for billing tag, got %v", result["data"])
	}
}

func TestSearchEntitiesByCapabilityEmpty(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Search for nonexistent capability — should return empty array
	resp := doJSON(t, "GET", "/api/v1/entities/search?capability=nonexistent_xyz", ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)

	result := parseResponse(t, resp)
	entities, ok := result["data"].([]interface{})
	if !ok {
		t.Fatalf("expected array data, got %T", result["data"])
	}
	if len(entities) != 0 {
		t.Fatalf("expected 0 entities for nonexistent capability, got %d", len(entities))
	}
}

func TestSearchEntitiesMissingParam(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "GET", "/api/v1/entities/search", ptr(token), nil)
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestSearchEntitiesExcludesDisabled(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create bot with skills
	resp := doJSON(t, "POST", "/api/v1/entities", ptr(token), map[string]interface{}{
		"name": "disabled-skill-bot",
		"metadata": map[string]interface{}{
			"capabilities": map[string]interface{}{
				"skills": []string{"deploy"},
			},
		},
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	entity, _ := data["entity"].(map[string]interface{})
	entityID := int(entity["id"].(float64))

	// Disable it
	resp = doJSON(t, "DELETE", fmt.Sprintf("/api/v1/entities/%d", entityID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)

	// Search — should NOT find disabled entity
	resp = doJSON(t, "GET", "/api/v1/entities/search?capability=deploy", ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)

	result := parseResponse(t, resp)
	entities, _ := result["data"].([]interface{})
	if len(entities) != 0 {
		t.Fatalf("expected 0 entities (disabled should be excluded), got %d", len(entities))
	}
}

// --- Task Handover Tests ---

func TestTaskHandoverStatusChange(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create conversation
	resp := doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title": "Handover Test",
	})
	assertStatus(t, resp, http.StatusCreated)
	convID := int(parseOK(t, resp)["id"].(float64))

	// Create task
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/tasks", convID), ptr(token), map[string]interface{}{
		"title": "Build login page",
	})
	assertStatus(t, resp, http.StatusCreated)
	taskID := int(parseOK(t, resp)["id"].(float64))

	// Verify task is pending
	resp = doJSON(t, "GET", fmt.Sprintf("/api/v1/tasks/%d", taskID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
	if parseOK(t, resp)["status"] != "pending" {
		t.Fatalf("expected task status=pending")
	}

	// Send task_handover message referencing the task
	handoverData := map[string]interface{}{
		"handover_type": "task_completion",
		"task_id":       taskID,
		"deliverables": []map[string]interface{}{
			{"type": "repo", "url": "https://github.com/example"},
		},
	}
	dataJSON, _ := json.Marshal(handoverData)
	var rawData json.RawMessage = dataJSON

	resp = doJSON(t, "POST", "/api/v1/messages/send", ptr(token), map[string]interface{}{
		"conversation_id": convID,
		"content_type":    "task_handover",
		"layers": map[string]interface{}{
			"summary": "Login page completed",
			"data":    rawData,
		},
	})
	assertStatus(t, resp, http.StatusCreated)

	// Verify task status changed to handed_over
	resp = doJSON(t, "GET", fmt.Sprintf("/api/v1/tasks/%d", taskID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
	taskData := parseOK(t, resp)
	if taskData["status"] != "handed_over" {
		t.Fatalf("expected task status=handed_over after handover message, got %v", taskData["status"])
	}
}

func TestTaskHandoverWithoutTaskID(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create conversation
	resp := doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title": "Handover No Task",
	})
	assertStatus(t, resp, http.StatusCreated)
	convID := int(parseOK(t, resp)["id"].(float64))

	// Send task_handover message without task_id — should succeed
	resp = doJSON(t, "POST", "/api/v1/messages/send", ptr(token), map[string]interface{}{
		"conversation_id": convID,
		"content_type":    "task_handover",
		"layers": map[string]interface{}{
			"summary": "General handover",
			"data": map[string]interface{}{
				"handover_type": "status_report",
			},
		},
	})
	assertStatus(t, resp, http.StatusCreated)

	msg := parseOK(t, resp)
	if msg["content_type"] != "task_handover" {
		t.Fatalf("expected content_type=task_handover, got %v", msg["content_type"])
	}
}

func TestTaskHandoverWrongConversation(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create conversation 1
	resp := doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title": "Conv 1",
	})
	assertStatus(t, resp, http.StatusCreated)
	convID1 := int(parseOK(t, resp)["id"].(float64))

	// Create conversation 2
	resp = doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title": "Conv 2",
	})
	assertStatus(t, resp, http.StatusCreated)
	convID2 := int(parseOK(t, resp)["id"].(float64))

	// Create task in conv 1
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/tasks", convID1), ptr(token), map[string]interface{}{
		"title": "Task in conv 1",
	})
	assertStatus(t, resp, http.StatusCreated)
	taskID := int(parseOK(t, resp)["id"].(float64))

	// Send handover in conv 2 referencing conv 1's task — should NOT change task status
	resp = doJSON(t, "POST", "/api/v1/messages/send", ptr(token), map[string]interface{}{
		"conversation_id": convID2,
		"content_type":    "task_handover",
		"layers": map[string]interface{}{
			"summary": "Cross-conv handover",
			"data": map[string]interface{}{
				"handover_type": "task_completion",
				"task_id":       taskID,
			},
		},
	})
	assertStatus(t, resp, http.StatusCreated)

	// Task should still be pending (different conversation)
	resp = doJSON(t, "GET", fmt.Sprintf("/api/v1/tasks/%d", taskID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
	if parseOK(t, resp)["status"] != "pending" {
		t.Fatal("task status should remain pending when handover is in a different conversation")
	}
}

func TestSendMessageWithMentionIntent(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create conversation
	resp := doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title": "Mention Intent Test",
	})
	assertStatus(t, resp, http.StatusCreated)
	convID := int(parseOK(t, resp)["id"].(float64))

	// Send message with mention_intent in layers.data
	resp = doJSON(t, "POST", "/api/v1/messages/send", ptr(token), map[string]interface{}{
		"conversation_id": convID,
		"layers": map[string]interface{}{
			"summary": "Please review this code",
			"data": map[string]interface{}{
				"mention_intent": map[string]interface{}{
					"type":        "review",
					"instruction": "Review the login module",
					"priority":    "high",
				},
			},
		},
	})
	assertStatus(t, resp, http.StatusCreated)

	// List messages and verify layers.data preserved
	resp = doJSON(t, "GET", fmt.Sprintf("/api/v1/conversations/%d/messages", convID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
	result := parseResponse(t, resp)
	data, _ := result["data"].(map[string]interface{})
	msgs, _ := data["messages"].([]interface{})
	if len(msgs) < 1 {
		t.Fatal("expected at least 1 message")
	}
	msg := msgs[0].(map[string]interface{})
	layers, _ := msg["layers"].(map[string]interface{})
	layerData, _ := layers["data"].(map[string]interface{})
	intent, _ := layerData["mention_intent"].(map[string]interface{})
	if intent["type"] != "review" {
		t.Fatalf("expected mention_intent.type=review, got %v", intent["type"])
	}
}
