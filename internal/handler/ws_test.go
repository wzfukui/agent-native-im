package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	gorillaWs "github.com/gorilla/websocket"
)

func TestWebSocketConnect(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	ts := newWSTestServer(t)
	defer ts.Close()

	wsURL := fmt.Sprintf("ws%s/api/v1/ws", ts.URL[len("http"):])
	conn, _, err := gorillaWs.DefaultDialer.Dial(wsURL, http.Header{"Authorization": []string{"Bearer " + token}})
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	defer conn.Close()

	// Send ping, expect pong (may need to skip entity.config first)
	msg := map[string]string{"type": "ping"}
	if err := conn.WriteJSON(msg); err != nil {
		t.Fatalf("ws write: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	for {
		var resp map[string]interface{}
		if err := conn.ReadJSON(&resp); err != nil {
			t.Fatalf("ws read: %v", err)
		}
		if resp["type"] == "entity.config" {
			continue // skip config push
		}
		if resp["type"] != "pong" {
			t.Fatalf("expected pong, got %v", resp["type"])
		}
		break
	}
}

func TestWebSocketNoToken(t *testing.T) {
	ts := newWSTestServer(t)
	defer ts.Close()

	// Without token — should get HTTP error, not upgrade
	wsURL := fmt.Sprintf("ws%s/api/v1/ws", ts.URL[len("http"):])
	_, resp, err := gorillaWs.DefaultDialer.Dial(wsURL, nil)
	if err == nil {
		t.Fatal("expected error for no token")
	}
	if resp != nil && resp.StatusCode != http.StatusUnauthorized {
		// WebSocket libraries may return different codes depending on how the server rejects
		t.Logf("got status %d (expected 401 or connection refused)", resp.StatusCode)
	}
}

func TestWebSocketSendMessage(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create a conversation first
	resp := doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title": "WS Message Test",
	})
	assertStatus(t, resp, http.StatusCreated)
	convData := parseOK(t, resp)
	convID := int(convData["id"].(float64))

	ts := newWSTestServer(t)
	defer ts.Close()

	wsURL := fmt.Sprintf("ws%s/api/v1/ws", ts.URL[len("http"):])
	conn, _, err := gorillaWs.DefaultDialer.Dial(wsURL, http.Header{"Authorization": []string{"Bearer " + token}})
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	defer conn.Close()

	// Drain entity.config message pushed on connect
	skipEntityConfig(t, conn)

	// Send a message via WebSocket
	sendMsg := map[string]interface{}{
		"type": "message.send",
		"data": map[string]interface{}{
			"conversation_id": convID,
			"content_type":    "text",
			"layers":          map[string]string{"summary": "Hello via WS"},
		},
	}
	if err := conn.WriteJSON(sendMsg); err != nil {
		t.Fatalf("ws write: %v", err)
	}

	// Should receive broadcast back (since we're the only participant)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, rawMsg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ws read: %v", err)
	}

	var received map[string]interface{}
	json.Unmarshal(rawMsg, &received)
	if received["type"] != "message.new" {
		t.Fatalf("expected type=message.new, got %v", received["type"])
	}
}

func TestWebSocketStreamProtocol(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create a bot — gets permanent key directly
	resp := doJSON(t, "POST", "/api/v1/entities", ptr(token), map[string]string{"name": "stream-receiver"})
	assertStatus(t, resp, http.StatusCreated)
	botData := parseOK(t, resp)
	botEntity, _ := botData["entity"].(map[string]interface{})
	botID := botEntity["id"].(float64)
	botKey, _ := botData["api_key"].(string)

	// Create conversation with bot
	resp = doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title":           "Stream Test",
		"conv_type":       "group",
		"participant_ids": []float64{botID},
	})
	assertStatus(t, resp, http.StatusCreated)
	convData := parseOK(t, resp)
	convID := int(convData["id"].(float64))

	// Set bot subscription to subscribe_all so it receives all messages
	ctx := context.Background()
	if err := testStore.UpdateSubscription(ctx, int64(convID), int64(botID), "subscribe_all"); err != nil {
		t.Fatalf("update subscription: %v", err)
	}

	ts := newWSTestServer(t)
	defer ts.Close()

	// Sender (admin) connects
	senderURL := fmt.Sprintf("ws%s/api/v1/ws", ts.URL[len("http"):])
	senderConn, _, err := gorillaWs.DefaultDialer.Dial(senderURL, http.Header{"Authorization": []string{"Bearer " + token}})
	if err != nil {
		t.Fatalf("ws dial sender: %v", err)
	}
	defer senderConn.Close()

	// Receiver (bot) connects with permanent key
	receiverURL := fmt.Sprintf("ws%s/api/v1/ws", ts.URL[len("http"):])
	receiverConn, _, err := gorillaWs.DefaultDialer.Dial(receiverURL, http.Header{"Authorization": []string{"Bearer " + botKey}})
	if err != nil {
		t.Fatalf("ws dial receiver: %v", err)
	}
	defer receiverConn.Close()

	// Drain entity.config messages on both connections
	skipEntityConfig(t, senderConn)
	skipEntityConfig(t, receiverConn)

	streamID := "test-stream-001"

	// 1. stream_start (ephemeral)
	senderConn.WriteJSON(map[string]interface{}{
		"type": "message.send",
		"data": map[string]interface{}{
			"conversation_id": convID,
			"stream_type":     "start",
			"stream_id":       streamID,
			"layers":          map[string]string{"summary": ""},
		},
	})

	// 2. stream_delta (ephemeral)
	senderConn.WriteJSON(map[string]interface{}{
		"type": "message.send",
		"data": map[string]interface{}{
			"conversation_id": convID,
			"stream_type":     "delta",
			"stream_id":       streamID,
			"layers":          map[string]string{"summary": "Partial content..."},
		},
	})

	// 3. stream_end (persisted)
	senderConn.WriteJSON(map[string]interface{}{
		"type": "message.send",
		"data": map[string]interface{}{
			"conversation_id": convID,
			"stream_type":     "end",
			"stream_id":       streamID,
			"content_type":    "markdown",
			"layers":          map[string]string{"summary": "Final complete content"},
		},
	})

	// Receiver should get: stream.start, stream.delta, message.new
	received := readWSMessages(t, receiverConn, 3, 3*time.Second)

	hasStreamStart := false
	hasStreamDelta := false
	hasMessageNew := false
	for _, msg := range received {
		switch msg["type"].(string) {
		case "stream.start":
			hasStreamStart = true
		case "stream.delta":
			hasStreamDelta = true
		case "message.new":
			hasMessageNew = true
		}
	}

	if !hasStreamStart {
		t.Error("receiver missing stream.start event")
	}
	if !hasStreamDelta {
		t.Error("receiver missing stream.delta event")
	}
	if !hasMessageNew {
		t.Error("receiver missing message.new event")
	}

	// Verify only the final message was persisted in DB
	msgResp := doJSON(t, "GET", fmt.Sprintf("/api/v1/conversations/%d/messages?limit=10", convID), ptr(token), nil)
	assertStatus(t, msgResp, http.StatusOK)
	msgData := parseOK(t, msgResp)
	msgs := msgData["messages"].([]interface{})
	if len(msgs) != 1 {
		t.Fatalf("expected exactly 1 persisted message, got %d", len(msgs))
	}

	persistedMsg := msgs[0].(map[string]interface{})
	if persistedMsg["stream_id"] != streamID {
		t.Fatalf("expected stream_id=%s, got %v", streamID, persistedMsg["stream_id"])
	}
}

func TestWebSocketPermanentKey(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create bot — gets permanent key
	resp := doJSON(t, "POST", "/api/v1/entities", ptr(token), map[string]string{"name": "ws-perm-bot"})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	apiKey, _ := data["api_key"].(string)

	ts := newWSTestServer(t)
	defer ts.Close()

	// Should be able to connect with permanent key
	wsURL := fmt.Sprintf("ws%s/api/v1/ws", ts.URL[len("http"):])
	conn, _, err := gorillaWs.DefaultDialer.Dial(wsURL, http.Header{"Authorization": []string{"Bearer " + apiKey}})
	if err != nil {
		t.Fatalf("ws dial with permanent key: %v", err)
	}
	conn.Close()
}

func TestBotDoesNotReceiveUnmentionedTwoMemberGroupMessage(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/entities", ptr(token), map[string]string{"name": "two-member-group-bot"})
	assertStatus(t, resp, http.StatusCreated)
	botData := parseOK(t, resp)
	botEntity, _ := botData["entity"].(map[string]interface{})
	botID := botEntity["id"].(float64)
	botKey, _ := botData["api_key"].(string)

	resp = doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title":           "Two Member Group",
		"conv_type":       "group",
		"participant_ids": []float64{botID},
	})
	assertStatus(t, resp, http.StatusCreated)
	convData := parseOK(t, resp)
	convID := int(convData["id"].(float64))

	ts := newWSTestServer(t)
	defer ts.Close()

	wsURL := fmt.Sprintf("ws%s/api/v1/ws", ts.URL[len("http"):])
	botConn, _, err := gorillaWs.DefaultDialer.Dial(wsURL, http.Header{"Authorization": []string{"Bearer " + botKey}})
	if err != nil {
		t.Fatalf("ws dial bot: %v", err)
	}
	defer botConn.Close()

	skipEntityConfig(t, botConn)

	resp = doJSON(t, "POST", "/api/v1/messages/send", ptr(token), map[string]interface{}{
		"conversation_id": convID,
		"content_type":    "text",
		"layers":          map[string]string{"summary": "No mention in two-member group"},
	})
	assertStatus(t, resp, http.StatusCreated)

	botConn.SetReadDeadline(time.Now().Add(1 * time.Second))
	var wsMsg map[string]interface{}
	if err := botConn.ReadJSON(&wsMsg); err == nil {
		t.Fatalf("bot should not receive unmentioned message in group chat, got %v", wsMsg["type"])
	}
}

func TestWebSocketRevokeEvent(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create a second user to receive the revoke event
	resp := doJSON(t, "POST", "/api/v1/admin/users", ptr(token), map[string]string{
		"username": "revoke-observer",
		"password": "Observer123",
	})
	assertStatus(t, resp, http.StatusCreated)
	observerData := parseOK(t, resp)
	observerID := observerData["id"].(float64)
	observerToken := login(t, "revoke-observer", "Observer123")

	// Create conversation with observer
	resp = doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title":           "Revoke Event Test",
		"conv_type":       "group",
		"participant_ids": []float64{observerID},
	})
	assertStatus(t, resp, http.StatusCreated)
	convData := parseOK(t, resp)
	convID := int(convData["id"].(float64))

	// Set observer to subscribe_all so they receive all events
	resp = doJSON(t, "PUT", fmt.Sprintf("/api/v1/conversations/%d/subscription", convID), ptr(observerToken), map[string]string{
		"mode": "subscribe_all",
	})
	assertStatus(t, resp, http.StatusOK)

	ts := newWSTestServer(t)
	defer ts.Close()

	// Observer connects via WS
	observerURL := fmt.Sprintf("ws%s/api/v1/ws", ts.URL[len("http"):])
	observerConn, _, err := gorillaWs.DefaultDialer.Dial(observerURL, http.Header{"Authorization": []string{"Bearer " + observerToken}})
	if err != nil {
		t.Fatalf("ws dial observer: %v", err)
	}
	defer observerConn.Close()

	// Drain entity.config
	skipEntityConfig(t, observerConn)

	// Send a message via HTTP
	resp = doJSON(t, "POST", "/api/v1/messages/send", ptr(token), map[string]interface{}{
		"conversation_id": convID,
		"content_type":    "text",
		"layers":          map[string]string{"summary": "to be revoked"},
	})
	assertStatus(t, resp, http.StatusCreated)
	msgData := parseOK(t, resp)
	msgID := int(msgData["id"].(float64))

	// Read the message.new event
	observerConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var newMsg map[string]interface{}
	observerConn.ReadJSON(&newMsg)

	// Revoke the message
	resp = doJSON(t, "DELETE", fmt.Sprintf("/api/v1/messages/%d", msgID), ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)

	// Read the revoke event — should be "message.revoked" (NOT "stream.message.revoked")
	observerConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var revokeMsg map[string]interface{}
	if err := observerConn.ReadJSON(&revokeMsg); err != nil {
		t.Fatalf("ws read revoke: %v", err)
	}

	if revokeMsg["type"] != "message.revoked" {
		t.Fatalf("expected type=message.revoked, got %v", revokeMsg["type"])
	}
}

// TestHumanAlwaysReceivesGroupMessages verifies that human users receive all messages
// in a group conversation regardless of subscription mode (they bypass mention_only filtering).
func TestHumanAlwaysReceivesGroupMessages(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create a second human user
	resp := doJSON(t, "POST", "/api/v1/admin/users", ptr(token), map[string]string{
		"username": "human2",
		"password": "Human2pass1",
	})
	assertStatus(t, resp, http.StatusCreated)
	human2Data := parseOK(t, resp)
	human2ID := human2Data["id"].(float64)
	human2Token := login(t, "human2", "Human2pass1")

	// Create group with human2
	resp = doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title":           "Human Test",
		"conv_type":       "group",
		"participant_ids": []float64{human2ID},
	})
	assertStatus(t, resp, http.StatusCreated)
	convData := parseOK(t, resp)
	convID := int(convData["id"].(float64))

	// human2's default subscription_mode is mention_only, but they should still get messages
	ts := newWSTestServer(t)
	defer ts.Close()

	wsURL := fmt.Sprintf("ws%s/api/v1/ws", ts.URL[len("http"):])
	conn, _, err := gorillaWs.DefaultDialer.Dial(wsURL, http.Header{"Authorization": []string{"Bearer " + human2Token}})
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	defer conn.Close()

	// Drain entity.config
	skipEntityConfig(t, conn)

	// Admin sends a message without mentioning human2
	resp = doJSON(t, "POST", "/api/v1/messages/send", ptr(token), map[string]interface{}{
		"conversation_id": convID,
		"layers":          map[string]string{"summary": "No mention here"},
	})
	assertStatus(t, resp, http.StatusCreated)

	// Human2 should still receive the message (humans bypass subscription filtering)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var wsMsg map[string]interface{}
	if err := conn.ReadJSON(&wsMsg); err != nil {
		t.Fatalf("human user should receive group message even without mention: %v", err)
	}
	if wsMsg["type"] != "message.new" {
		t.Fatalf("expected type=message.new, got %v", wsMsg["type"])
	}
}

func TestOwnerReceivesBotPresenceWithoutSharedConversation(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/entities", ptr(token), map[string]string{"name": "presence-owned-bot"})
	assertStatus(t, resp, http.StatusCreated)
	botData := parseOK(t, resp)
	botEntity, _ := botData["entity"].(map[string]interface{})
	botID := int64(botEntity["id"].(float64))
	botKey, _ := botData["api_key"].(string)
	if botKey == "" {
		t.Fatal("expected bot api_key")
	}

	ts := newWSTestServer(t)
	defer ts.Close()
	wsURL := fmt.Sprintf("ws%s/api/v1/ws", ts.URL[len("http"):])

	ownerConn, _, err := gorillaWs.DefaultDialer.Dial(wsURL, http.Header{"Authorization": []string{"Bearer " + token}})
	if err != nil {
		t.Fatalf("owner ws dial: %v", err)
	}
	defer ownerConn.Close()
	skipEntityConfig(t, ownerConn)

	botConn, _, err := gorillaWs.DefaultDialer.Dial(wsURL, http.Header{"Authorization": []string{"Bearer " + botKey}})
	if err != nil {
		t.Fatalf("bot ws dial: %v", err)
	}
	defer botConn.Close()

	deadline := time.Now().Add(3 * time.Second)
	ownerConn.SetReadDeadline(deadline)
	for {
		var msg map[string]interface{}
		if err := ownerConn.ReadJSON(&msg); err != nil {
			t.Fatalf("owner should receive bot presence update: %v", err)
		}
		if msg["type"] != "entity.online" {
			continue
		}
		data, _ := msg["data"].(map[string]interface{})
		if int64(data["entity_id"].(float64)) != botID {
			continue
		}
		return
	}
}

func TestBotDoesNotReceiveSystemTaskMessageAsChatInput(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/entities", ptr(token), map[string]string{"name": "task-system-bot"})
	assertStatus(t, resp, http.StatusCreated)
	botData := parseOK(t, resp)
	botEntity, _ := botData["entity"].(map[string]interface{})
	botID := botEntity["id"].(float64)
	botKey, _ := botData["api_key"].(string)
	if botKey == "" {
		t.Fatal("expected bot api_key")
	}

	resp = doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title":           "Task System Test",
		"conv_type":       "group",
		"participant_ids": []float64{botID},
	})
	assertStatus(t, resp, http.StatusCreated)
	convData := parseOK(t, resp)
	convID := int(convData["id"].(float64))

	resp = doJSON(t, "PUT", fmt.Sprintf("/api/v1/conversations/%d/subscription", convID), ptr(botKey), map[string]string{
		"mode": "subscribe_all",
	})
	assertStatus(t, resp, http.StatusOK)

	ts := newWSTestServer(t)
	defer ts.Close()
	wsURL := fmt.Sprintf("ws%s/api/v1/ws", ts.URL[len("http"):])

	botConn, _, err := gorillaWs.DefaultDialer.Dial(wsURL, http.Header{"Authorization": []string{"Bearer " + botKey}})
	if err != nil {
		t.Fatalf("bot ws dial: %v", err)
	}
	defer botConn.Close()
	skipEntityConfig(t, botConn)

	resp = doJSON(t, "POST", fmt.Sprintf("/api/v1/conversations/%d/tasks", convID), ptr(token), map[string]interface{}{
		"title": "Install GitHub CLI",
	})
	assertStatus(t, resp, http.StatusCreated)

	msgs := readWSMessages(t, botConn, 4, 1500*time.Millisecond)
	foundTaskUpdated := false
	for _, msg := range msgs {
		switch msg["type"] {
		case "task.updated":
			foundTaskUpdated = true
		case "message.new":
			t.Fatalf("bot should not receive task system messages as chat input: %+v", msg)
		}
	}
	if !foundTaskUpdated {
		t.Fatal("expected bot to still receive task.updated event")
	}
}

// skipEntityConfig drains initial WS messages (presence, config) until entity.config is found or timeout.
func skipEntityConfig(t *testing.T, conn *gorillaWs.Conn) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	conn.SetReadDeadline(deadline)
	for {
		var msg map[string]interface{}
		if err := conn.ReadJSON(&msg); err != nil {
			t.Logf("skipEntityConfig: no entity.config received before timeout: %v", err)
			return
		}
		if msg["type"] == "entity.config" {
			return // found it, done
		}
		// Skip other initial messages (entity.online, presence, etc.)
		t.Logf("skipEntityConfig: skipping %v", msg["type"])
	}
}

// readWSMessages reads up to n messages from the WebSocket within the timeout.
func readWSMessages(t *testing.T, conn *gorillaWs.Conn, n int, timeout time.Duration) []map[string]interface{} {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var messages []map[string]interface{}

	for len(messages) < n {
		conn.SetReadDeadline(deadline)
		_, raw, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var msg map[string]interface{}
		json.Unmarshal(raw, &msg)
		messages = append(messages, msg)
	}

	return messages
}
