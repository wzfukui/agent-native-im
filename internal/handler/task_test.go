package handler_test

import (
	"fmt"
	"net/http"
	"testing"
)

func TestCreateTask(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create conversation
	resp := doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title": "Task Test",
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	convID := int(data["id"].(float64))

	// Create task
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/tasks", convID), ptr(token), map[string]interface{}{
		"title":       "Fix the bug",
		"description": "It crashes on login",
		"priority":    "high",
	})
	assertStatus(t, resp, http.StatusCreated)
	data = parseOK(t, resp)
	if data["title"] != "Fix the bug" {
		t.Fatalf("expected title 'Fix the bug', got %v", data["title"])
	}
	if data["priority"] != "high" {
		t.Fatalf("expected priority 'high', got %v", data["priority"])
	}
	if data["status"] != "pending" {
		t.Fatalf("expected status 'pending', got %v", data["status"])
	}
}

func TestCreateTaskMissingTitle(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title": "Task Test",
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	convID := int(data["id"].(float64))

	// Missing title should fail
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/tasks", convID), ptr(token), map[string]interface{}{
		"description": "no title",
	})
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestListTasks(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title": "Task List Test",
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	convID := int(data["id"].(float64))

	// Create two tasks
	doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/tasks", convID), ptr(token), map[string]interface{}{
		"title": "Task A",
	})
	doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/tasks", convID), ptr(token), map[string]interface{}{
		"title": "Task B",
	})

	// List all
	resp = doJSON(t, "GET", fmt.Sprintf("/api/v1/conversations/%d/tasks", convID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
	result := parseResponse(t, resp)
	tasks, ok := result["data"].([]interface{})
	if !ok || len(tasks) < 2 {
		t.Fatalf("expected at least 2 tasks, got %v", result["data"])
	}
}

func TestListTasksFilterByStatus(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title": "Filter Test",
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	convID := int(data["id"].(float64))

	// Create task
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/tasks", convID), ptr(token), map[string]interface{}{
		"title": "Pending Task",
	})
	assertStatus(t, resp, http.StatusCreated)

	// List with status filter — should find 1 pending
	resp = doJSON(t, "GET", fmt.Sprintf("/api/v1/conversations/%d/tasks?status=pending", convID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
	result := parseResponse(t, resp)
	tasks := result["data"].([]interface{})
	if len(tasks) != 1 {
		t.Fatalf("expected 1 pending task, got %d", len(tasks))
	}

	// Filter by done — should be empty
	resp = doJSON(t, "GET", fmt.Sprintf("/api/v1/conversations/%d/tasks?status=done", convID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
	result = parseResponse(t, resp)
	if doneTasks, ok := result["data"].([]interface{}); ok && len(doneTasks) != 0 {
		t.Fatalf("expected 0 done tasks, got %d", len(doneTasks))
	}
}

func TestGetTask(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title": "Get Task Test",
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	convID := int(data["id"].(float64))

	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/tasks", convID), ptr(token), map[string]interface{}{
		"title": "A Single Task",
	})
	assertStatus(t, resp, http.StatusCreated)
	data = parseOK(t, resp)
	taskID := int(data["id"].(float64))

	// Get task by ID
	resp = doJSON(t, "GET", fmt.Sprintf("/api/v1/tasks/%d", taskID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
	data = parseOK(t, resp)
	if data["title"] != "A Single Task" {
		t.Fatalf("expected title 'A Single Task', got %v", data["title"])
	}
}

func TestUpdateTask(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title": "Update Task Test",
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	convID := int(data["id"].(float64))

	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/tasks", convID), ptr(token), map[string]interface{}{
		"title": "Original Title",
	})
	assertStatus(t, resp, http.StatusCreated)
	data = parseOK(t, resp)
	taskID := int(data["id"].(float64))

	// Update title and status
	resp = doJSON(t, "PUT", fmt.Sprintf("/api/v1/tasks/%d", taskID), ptr(token), map[string]interface{}{
		"title":  "Updated Title",
		"status": "in_progress",
	})
	assertStatus(t, resp, http.StatusOK)
	data = parseOK(t, resp)
	if data["title"] != "Updated Title" {
		t.Fatalf("expected 'Updated Title', got %v", data["title"])
	}
	if data["status"] != "in_progress" {
		t.Fatalf("expected 'in_progress', got %v", data["status"])
	}
}

func TestUpdateTaskComplete(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title": "Complete Task Test",
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	convID := int(data["id"].(float64))

	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/tasks", convID), ptr(token), map[string]interface{}{
		"title": "Task to Complete",
	})
	assertStatus(t, resp, http.StatusCreated)
	data = parseOK(t, resp)
	taskID := int(data["id"].(float64))

	// Mark done
	resp = doJSON(t, "PUT", fmt.Sprintf("/api/v1/tasks/%d", taskID), ptr(token), map[string]interface{}{
		"status": "done",
	})
	assertStatus(t, resp, http.StatusOK)
	data = parseOK(t, resp)
	if data["status"] != "done" {
		t.Fatalf("expected 'done', got %v", data["status"])
	}
	if data["completed_at"] == nil {
		t.Fatal("expected completed_at to be set")
	}
}

func TestDeleteTask(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title": "Delete Task Test",
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	convID := int(data["id"].(float64))

	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/tasks", convID), ptr(token), map[string]interface{}{
		"title": "Task to Delete",
	})
	assertStatus(t, resp, http.StatusCreated)
	data = parseOK(t, resp)
	taskID := int(data["id"].(float64))

	// Delete
	resp = doJSON(t, "DELETE", fmt.Sprintf("/api/v1/tasks/%d", taskID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)

	// Get should 404
	resp = doJSON(t, "GET", fmt.Sprintf("/api/v1/tasks/%d", taskID), ptr(token), nil)
	assertStatus(t, resp, http.StatusNotFound)
}

func TestTaskNotParticipant(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create second user
	doJSON(t, "POST", "/api/v1/admin/users", ptr(token), map[string]string{
		"username": "outsider",
		"password": "outsider123",
	})
	outsiderToken := login(t, "outsider", "outsider123")

	resp := doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title": "Private Conv",
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	convID := int(data["id"].(float64))

	// Outsider tries to create task — should fail
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/tasks", convID), ptr(outsiderToken), map[string]interface{}{
		"title": "Sneaky Task",
	})
	assertStatus(t, resp, http.StatusForbidden)
}
