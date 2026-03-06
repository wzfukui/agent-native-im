package handler_test

import (
	"fmt"
	"net/http"
	"testing"
)

func TestCreateChangeRequest(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create second user (member)
	resp := doJSON(t, "POST", "/api/v1/admin/users", ptr(token), map[string]string{
		"username": "member1",
		"password": "Member123",
	})
	assertStatus(t, resp, http.StatusCreated)
	memberData := parseOK(t, resp)
	memberID := int(memberData["id"].(float64))
	memberToken := login(t, "member1", "Member123")

	// Create group with member
	resp = doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title":           "CR Test Group",
		"conv_type":       "group",
		"participant_ids": []float64{float64(memberID)},
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	convID := int(data["id"].(float64))

	// Member creates a change request
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/change-requests", convID), ptr(memberToken), map[string]interface{}{
		"field":     "title",
		"new_value": "Better Group Name",
	})
	assertStatus(t, resp, http.StatusCreated)
	data = parseOK(t, resp)
	if data["field"] != "title" {
		t.Fatalf("expected field 'title', got %v", data["field"])
	}
	if data["status"] != "pending" {
		t.Fatalf("expected status 'pending', got %v", data["status"])
	}
}

func TestListChangeRequests(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/admin/users", ptr(token), map[string]string{
		"username": "member1",
		"password": "Member123",
	})
	assertStatus(t, resp, http.StatusCreated)
	memberData := parseOK(t, resp)
	memberID := int(memberData["id"].(float64))
	memberToken := login(t, "member1", "Member123")

	resp = doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title":           "CR List Test",
		"conv_type":       "group",
		"participant_ids": []float64{float64(memberID)},
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	convID := int(data["id"].(float64))

	// Create two change requests
	doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/change-requests", convID), ptr(memberToken), map[string]interface{}{
		"field":     "title",
		"new_value": "Name 1",
	})
	doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/change-requests", convID), ptr(memberToken), map[string]interface{}{
		"field":     "description",
		"new_value": "New desc",
	})

	// List all
	resp = doJSON(t, "GET", fmt.Sprintf("/api/v1/conversations/%d/change-requests", convID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
	result := parseResponse(t, resp)
	requests, ok := result["data"].([]interface{})
	if !ok || len(requests) < 2 {
		t.Fatalf("expected at least 2 change requests, got %v", result["data"])
	}
}

func TestApproveChangeRequest(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/admin/users", ptr(token), map[string]string{
		"username": "member1",
		"password": "Member123",
	})
	assertStatus(t, resp, http.StatusCreated)
	memberData := parseOK(t, resp)
	memberID := int(memberData["id"].(float64))
	memberToken := login(t, "member1", "Member123")

	resp = doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title":           "Approve CR Test",
		"conv_type":       "group",
		"participant_ids": []float64{float64(memberID)},
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	convID := int(data["id"].(float64))

	// Member requests title change
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/change-requests", convID), ptr(memberToken), map[string]interface{}{
		"field":     "title",
		"new_value": "Approved Title",
	})
	assertStatus(t, resp, http.StatusCreated)
	data = parseOK(t, resp)
	crID := int(data["id"].(float64))

	// Owner approves
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/change-requests/%d/approve", convID, crID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)

	// Verify conversation title was updated
	resp = doJSON(t, "GET", fmt.Sprintf("/api/v1/conversations/%d", convID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
	data = parseOK(t, resp)
	if data["title"] != "Approved Title" {
		t.Fatalf("expected conversation title 'Approved Title', got %v", data["title"])
	}
}

func TestRejectChangeRequest(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/admin/users", ptr(token), map[string]string{
		"username": "member1",
		"password": "Member123",
	})
	assertStatus(t, resp, http.StatusCreated)
	memberData := parseOK(t, resp)
	memberID := int(memberData["id"].(float64))
	memberToken := login(t, "member1", "Member123")

	resp = doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title":           "Reject CR Test",
		"conv_type":       "group",
		"participant_ids": []float64{float64(memberID)},
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	convID := int(data["id"].(float64))

	// Member requests title change
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/change-requests", convID), ptr(memberToken), map[string]interface{}{
		"field":     "title",
		"new_value": "Bad Title",
	})
	assertStatus(t, resp, http.StatusCreated)
	data = parseOK(t, resp)
	crID := int(data["id"].(float64))

	// Owner rejects
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/change-requests/%d/reject", convID, crID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)

	// Verify conversation title unchanged
	resp = doJSON(t, "GET", fmt.Sprintf("/api/v1/conversations/%d", convID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
	data = parseOK(t, resp)
	if data["title"] != "Reject CR Test" {
		t.Fatalf("expected conversation title unchanged, got %v", data["title"])
	}
}

func TestChangeRequestMemberCannotApprove(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/admin/users", ptr(token), map[string]string{
		"username": "member1",
		"password": "Member123",
	})
	assertStatus(t, resp, http.StatusCreated)
	memberData := parseOK(t, resp)
	memberID := int(memberData["id"].(float64))
	memberToken := login(t, "member1", "Member123")

	resp = doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title":           "No Approve Test",
		"conv_type":       "group",
		"participant_ids": []float64{float64(memberID)},
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	convID := int(data["id"].(float64))

	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/change-requests", convID), ptr(memberToken), map[string]interface{}{
		"field":     "title",
		"new_value": "My Title",
	})
	assertStatus(t, resp, http.StatusCreated)
	data = parseOK(t, resp)
	crID := int(data["id"].(float64))

	// Member tries to approve own request — should fail
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/change-requests/%d/approve", convID, crID), ptr(memberToken), nil)
	assertStatus(t, resp, http.StatusForbidden)
}

func TestChangeRequestNotParticipant(t *testing.T) {
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

	// Outsider tries to create change request
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/change-requests", convID), ptr(outsiderToken), map[string]interface{}{
		"field":     "title",
		"new_value": "Hacked",
	})
	assertStatus(t, resp, http.StatusForbidden)
}
