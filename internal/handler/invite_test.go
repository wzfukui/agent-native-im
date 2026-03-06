package handler_test

import (
	"fmt"
	"net/http"
	"testing"
)

// --- Invite Link CRUD ---

func TestCreateInviteLink(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create conversation
	resp := doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title": "Invite Test",
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	convID := int(data["id"].(float64))

	// Create invite link
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/invite", convID), ptr(token), map[string]interface{}{
		"max_uses":   10,
		"expires_in": 3600, // 1 hour
	})
	assertStatus(t, resp, http.StatusOK)
	data = parseOK(t, resp)
	code, ok := data["code"].(string)
	if !ok || code == "" {
		t.Fatal("expected invite code in response")
	}
	if data["conversation_id"] == nil {
		t.Fatal("expected conversation_id in response")
	}
}

func TestCreateInviteLinkMemberForbidden(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create a regular member
	resp := doJSON(t, "POST", "/api/v1/admin/users", ptr(token), map[string]string{
		"username": "member",
		"password": "Member123",
	})
	assertStatus(t, resp, http.StatusCreated)
	memberData := parseOK(t, resp)
	memberID := int(memberData["id"].(float64))
	memberToken := login(t, "member", "Member123")

	// Create conversation with member
	resp = doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title":           "Invite Member Test",
		"conv_type":       "group",
		"participant_ids": []float64{float64(memberID)},
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	convID := int(data["id"].(float64))

	// Member tries to create invite — should fail (owner/admin only)
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/invite", convID), ptr(memberToken), map[string]interface{}{})
	assertStatus(t, resp, http.StatusForbidden)
}

func TestListInviteLinks(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title": "List Invites",
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	convID := int(data["id"].(float64))

	// Create 2 invite links
	doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/invite", convID), ptr(token), nil)
	doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/invite", convID), ptr(token), nil)

	// List invite links
	resp = doJSON(t, "GET", fmt.Sprintf("/api/v1/conversations/%d/invites", convID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
	result := parseResponse(t, resp)
	links, ok := result["data"].([]interface{})
	if !ok || len(links) < 2 {
		t.Fatalf("expected at least 2 invite links, got %v", result["data"])
	}
}

func TestGetInviteInfo(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title": "Invite Info",
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	convID := int(data["id"].(float64))

	// Create invite
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/invite", convID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
	data = parseOK(t, resp)
	code := data["code"].(string)

	// Get invite info
	resp = doJSON(t, "GET", fmt.Sprintf("/api/v1/invite/%s", code), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
	data = parseOK(t, resp)
	if data["invite"] == nil {
		t.Fatal("expected invite in response")
	}
	if data["conversation"] == nil {
		t.Fatal("expected conversation in response")
	}
}

func TestJoinViaInvite(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create a user to join
	doJSON(t, "POST", "/api/v1/admin/users", ptr(token), map[string]string{
		"username": "joiner",
		"password": "Joiner123",
	})
	joinerToken := login(t, "joiner", "Joiner123")

	// Create conversation
	resp := doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title": "Join Via Invite",
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	convID := int(data["id"].(float64))

	// Create invite
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/invite", convID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
	data = parseOK(t, resp)
	code := data["code"].(string)

	// Joiner joins via invite
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/invite/%s/join", code), ptr(joinerToken), nil)
	assertStatus(t, resp, http.StatusOK)

	// Joiner should now see the conversation
	resp = doJSON(t, "GET", "/api/v1/conversations", ptr(joinerToken), nil)
	assertStatus(t, resp, http.StatusOK)
	result := parseResponse(t, resp)
	convs, _ := result["data"].([]interface{})
	found := false
	for _, c := range convs {
		conv := c.(map[string]interface{})
		if int(conv["id"].(float64)) == convID {
			found = true
		}
	}
	if !found {
		t.Fatal("joiner should see the conversation after joining via invite")
	}
}

func TestJoinViaInviteAlreadyParticipant(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title": "Already In",
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	convID := int(data["id"].(float64))

	// Create invite
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/invite", convID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
	data = parseOK(t, resp)
	code := data["code"].(string)

	// Admin tries to join own conversation — already participant
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/invite/%s/join", code), ptr(token), nil)
	assertStatus(t, resp, http.StatusConflict)
}

func TestJoinViaInviteMaxUsesReached(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create 2 users
	doJSON(t, "POST", "/api/v1/admin/users", ptr(token), map[string]string{
		"username": "user1",
		"password": "User1pass1",
	})
	user1Token := login(t, "user1", "User1pass1")

	doJSON(t, "POST", "/api/v1/admin/users", ptr(token), map[string]string{
		"username": "user2",
		"password": "User2pass1",
	})
	user2Token := login(t, "user2", "User2pass1")

	resp := doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title": "Max Uses",
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	convID := int(data["id"].(float64))

	// Create invite with max_uses=1
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/invite", convID), ptr(token), map[string]interface{}{
		"max_uses": 1,
	})
	assertStatus(t, resp, http.StatusOK)
	data = parseOK(t, resp)
	code := data["code"].(string)

	// First join succeeds
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/invite/%s/join", code), ptr(user1Token), nil)
	assertStatus(t, resp, http.StatusOK)

	// Second join fails — max uses reached
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/invite/%s/join", code), ptr(user2Token), nil)
	assertStatus(t, resp, http.StatusGone)
}

func TestDeleteInviteLink(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title": "Delete Invite",
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	convID := int(data["id"].(float64))

	// Create invite
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/invite", convID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
	data = parseOK(t, resp)
	inviteID := int(data["id"].(float64))

	// Delete invite
	resp = doJSON(t, "DELETE", fmt.Sprintf("/api/v1/invites/%d", inviteID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
}

func TestGetInviteInfoNotFound(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "GET", "/api/v1/invite/nonexistent_code_12345", ptr(token), nil)
	assertStatus(t, resp, http.StatusNotFound)
}
