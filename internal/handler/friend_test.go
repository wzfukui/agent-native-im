package handler_test

import (
	"fmt"
	"net/http"
	"testing"
)

func TestFriendRequestLifecycle(t *testing.T) {
	truncateAll(t)
	adminToken := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/admin/users", ptr(adminToken), map[string]string{
		"username": "friend-user-a",
		"password": "Friendpass1",
	})
	assertStatus(t, resp, http.StatusCreated)
	userAToken := login(t, "friend-user-a", "Friendpass1")

	resp = doJSON(t, "POST", "/api/v1/admin/users", ptr(adminToken), map[string]string{
		"username": "friend-user-b",
		"password": "Friendpass1",
	})
	assertStatus(t, resp, http.StatusCreated)
	userBID := int(parseOK(t, resp)["id"].(float64))
	userBToken := login(t, "friend-user-b", "Friendpass1")

	resp = doJSON(t, "POST", "/api/v1/friends/requests", ptr(userAToken), map[string]interface{}{
		"target_entity_id": userBID,
	})
	assertStatus(t, resp, http.StatusCreated)
	reqID := int(parseOK(t, resp)["id"].(float64))

	resp = doJSON(t, "GET", fmt.Sprintf("/api/v1/friends/requests?direction=incoming"), ptr(userBToken), nil)
	assertStatus(t, resp, http.StatusOK)
	incoming := parseResponse(t, resp)["data"].([]interface{})
	if len(incoming) != 1 {
		t.Fatalf("expected 1 incoming request, got %d", len(incoming))
	}

	resp = doJSON(t, "GET", "/api/v1/friends/requests?direction=outgoing&status=pending", ptr(userAToken), nil)
	assertStatus(t, resp, http.StatusOK)
	outgoing := parseResponse(t, resp)["data"].([]interface{})
	if len(outgoing) != 1 {
		t.Fatalf("expected 1 pending outgoing request, got %d", len(outgoing))
	}

	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/friends/requests/%d/accept", reqID), ptr(userBToken), nil)
	assertStatus(t, resp, http.StatusOK)

	resp = doJSON(t, "GET", "/api/v1/friends", ptr(userAToken), nil)
	assertStatus(t, resp, http.StatusOK)
	friends := parseResponse(t, resp)["data"].([]interface{})
	if len(friends) != 1 {
		t.Fatalf("expected 1 friend, got %d", len(friends))
	}
	friend := friends[0].(map[string]interface{})
	if int(friend["id"].(float64)) != userBID {
		t.Fatalf("expected friend id %d, got %v", userBID, friend["id"])
	}

	resp = doJSON(t, "DELETE", fmt.Sprintf("/api/v1/friends/%d", userBID), ptr(userAToken), nil)
	assertStatus(t, resp, http.StatusOK)

	resp = doJSON(t, "GET", "/api/v1/friends", ptr(userAToken), nil)
	assertStatus(t, resp, http.StatusOK)
	friends = parseResponse(t, resp)["data"].([]interface{})
	if len(friends) != 0 {
		t.Fatalf("expected 0 friends after delete, got %d", len(friends))
	}
}

func TestDirectConversationRequiresFriendshipUnlessBotOptIn(t *testing.T) {
	truncateAll(t)
	adminToken := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/admin/users", ptr(adminToken), map[string]string{
		"username": "dm-user-a",
		"password": "Friendpass1",
	})
	assertStatus(t, resp, http.StatusCreated)
	userAToken := login(t, "dm-user-a", "Friendpass1")

	resp = doJSON(t, "POST", "/api/v1/admin/users", ptr(adminToken), map[string]string{
		"username": "dm-user-b",
		"password": "Friendpass1",
	})
	assertStatus(t, resp, http.StatusCreated)
	userBID := int(parseOK(t, resp)["id"].(float64))

	resp = doJSON(t, "POST", "/api/v1/conversations", ptr(userAToken), map[string]interface{}{
		"title":           "Not Allowed",
		"conv_type":       "direct",
		"participant_ids": []int{userBID},
	})
	assertStatus(t, resp, http.StatusForbidden)

	resp = doJSON(t, "POST", "/api/v1/friends/requests", ptr(userAToken), map[string]interface{}{
		"target_entity_id": userBID,
	})
	assertStatus(t, resp, http.StatusCreated)
	reqID := int(parseOK(t, resp)["id"].(float64))

	userBToken := login(t, "dm-user-b", "Friendpass1")
	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/friends/requests/%d/accept", reqID), ptr(userBToken), nil)
	assertStatus(t, resp, http.StatusOK)

	resp = doJSON(t, "POST", "/api/v1/conversations", ptr(userAToken), map[string]interface{}{
		"title":           "Allowed",
		"conv_type":       "direct",
		"participant_ids": []int{userBID},
	})
	assertStatus(t, resp, http.StatusCreated)

	resp = doJSON(t, "POST", "/api/v1/entities", ptr(adminToken), map[string]string{"name": "public-helper"})
	assertStatus(t, resp, http.StatusCreated)
	botData := parseOK(t, resp)["entity"].(map[string]interface{})
	botID := int(botData["id"].(float64))

	resp = doJSON(t, "PUT", fmt.Sprintf("/api/v1/entities/%d", botID), ptr(adminToken), map[string]interface{}{
		"discoverability":       "platform_public",
		"allow_non_friend_chat": true,
	})
	assertStatus(t, resp, http.StatusOK)

	resp = doJSON(t, "POST", "/api/v1/conversations", ptr(userAToken), map[string]interface{}{
		"title":           "Support Bot",
		"conv_type":       "direct",
		"participant_ids": []int{botID},
	})
	assertStatus(t, resp, http.StatusCreated)
}

func TestDiscoverableEntitySearchRequiresExactMatch(t *testing.T) {
	truncateAll(t)
	adminToken := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/entities", ptr(adminToken), map[string]string{
		"name":   "support helper",
		"bot_id": "bot_support_helper",
	})
	assertStatus(t, resp, http.StatusCreated)
	entity := parseOK(t, resp)["entity"].(map[string]interface{})
	botID := int(entity["id"].(float64))
	publicID := entity["public_id"].(string)

	resp = doJSON(t, "PUT", fmt.Sprintf("/api/v1/entities/%d", botID), ptr(adminToken), map[string]interface{}{
		"discoverability": "platform_public",
	})
	assertStatus(t, resp, http.StatusOK)

	resp = doJSON(t, "GET", "/api/v1/entities/discover?q=bot_support&limit=20", ptr(adminToken), nil)
	assertStatus(t, resp, http.StatusOK)
	results := parseResponse(t, resp)["data"].([]interface{})
	if len(results) != 0 {
		t.Fatalf("expected partial bot_id search to return 0 results, got %d", len(results))
	}

	resp = doJSON(t, "GET", "/api/v1/entities/discover?q=bot_support_helper&limit=20", ptr(adminToken), nil)
	assertStatus(t, resp, http.StatusOK)
	results = parseResponse(t, resp)["data"].([]interface{})
	if len(results) != 1 {
		t.Fatalf("expected exact bot_id search to return 1 result, got %d", len(results))
	}

	resp = doJSON(t, "GET", "/api/v1/entities/discover?q="+publicID+"&limit=20", ptr(adminToken), nil)
	assertStatus(t, resp, http.StatusOK)
	results = parseResponse(t, resp)["data"].([]interface{})
	if len(results) != 1 {
		t.Fatalf("expected exact public_id search to return 1 result, got %d", len(results))
	}
}
