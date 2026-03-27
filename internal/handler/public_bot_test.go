package handler_test

import (
	"fmt"
	"net/http"
	"testing"
)

func TestPublicBotAccessLinkSession(t *testing.T) {
	truncateAll(t)
	adminToken := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/admin/users", ptr(adminToken), map[string]string{
		"username": "public-bot-owner",
		"password": "Friendpass1",
	})
	assertStatus(t, resp, http.StatusCreated)
	ownerToken := login(t, "public-bot-owner", "Friendpass1")

	resp = doJSON(t, "POST", "/api/v1/entities", ptr(ownerToken), map[string]string{
		"name":   "public-helper",
		"bot_id": "bot_public_helper",
	})
	assertStatus(t, resp, http.StatusCreated)
	bot := parseOK(t, resp)["entity"].(map[string]interface{})
	botID := int(bot["id"].(float64))

	resp = doJSON(t, "PUT", fmt.Sprintf("/api/v1/entities/%d", botID), ptr(ownerToken), map[string]interface{}{
		"discoverability":         "external_public",
		"allow_non_friend_chat":   true,
		"require_access_password": true,
		"access_password":         "guestpass1",
	})
	assertStatus(t, resp, http.StatusOK)

	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/bots/%d/access-links", botID), ptr(ownerToken), map[string]interface{}{
		"label": "Support Entry",
	})
	assertStatus(t, resp, http.StatusCreated)
	linkCode := parseOK(t, resp)["code"].(string)

	resp = doJSON(t, "GET", "/api/v1/public/bots/bot_public_helper?code="+linkCode, nil, nil)
	assertStatus(t, resp, http.StatusOK)

	resp = doJSON(t, "POST", "/api/v1/public/bots/bot_public_helper/session", nil, map[string]interface{}{
		"access_code":  linkCode,
		"password":     "guestpass1",
		"display_name": "Website Guest",
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	visitorToken := data["token"].(string)
	conv := data["conversation"].(map[string]interface{})
	convID := int(conv["id"].(float64))

	resp = doJSON(t, "POST", "/api/v1/messages/send", ptr(visitorToken), map[string]interface{}{
		"conversation_id": convID,
		"layers": map[string]interface{}{
			"summary": "hello from visitor",
		},
	})
	assertStatus(t, resp, http.StatusCreated)

	resp = doJSON(t, "GET", fmt.Sprintf("/api/v1/conversations/%d/messages", convID), ptr(visitorToken), nil)
	assertStatus(t, resp, http.StatusOK)

	resp = doJSON(t, "GET", "/api/v1/notifications?status=unread", ptr(ownerToken), nil)
	assertStatus(t, resp, http.StatusOK)
	notifications := parseResponse(t, resp)["data"].([]interface{})
	found := false
	for _, item := range notifications {
		notification := item.(map[string]interface{})
		if notification["kind"] == "public.bot_session_created" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected public.bot_session_created notification for bot owner")
	}
}
