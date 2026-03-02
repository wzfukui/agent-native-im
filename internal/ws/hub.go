package ws

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	"github.com/wzfukui/agent-native-im/internal/model"
	"github.com/wzfukui/agent-native-im/internal/store"
)

// PushFunc is called for offline users when a message is broadcast.
// entityID is the recipient, msg is the message being broadcast.
type PushFunc func(entityID int64, msg *model.Message)

type Hub struct {
	store      store.Store
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client

	// conversation -> set of clients
	convClients map[int64]map[*Client]bool
	mu          sync.RWMutex // protects clients AND convClients

	// Long polling waiters: entityID -> channels
	waiters  map[int64][]chan struct{}
	waiterMu sync.Mutex

	// Push notification callback for offline users
	OnPush PushFunc
}

func NewHub(st store.Store) *Hub {
	return &Hub{
		store:       st,
		clients:     make(map[*Client]bool),
		register:    make(chan *Client),
		unregister:  make(chan *Client),
		convClients: make(map[int64]map[*Client]bool),
		waiters:     make(map[int64][]chan struct{}),
	}
}

func (h *Hub) Run() {
	log.Println("ws: hub started")
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			wasOnline := h.isOnlineLocked(client.entityID)
			h.clients[client] = true
			h.subscribeClientLocked(client)
			total := len(h.clients)
			h.mu.Unlock()

			log.Printf("ws: entity %d connected (total: %d)", client.entityID, total)

			if !wasOnline {
				h.broadcastPresence(client.entityID, true)
			}

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				h.unsubscribeClientLocked(client)
				stillOnline := h.isOnlineLocked(client.entityID)
				total := len(h.clients)
				h.mu.Unlock()

				log.Printf("ws: entity %d disconnected (total: %d)", client.entityID, total)

				if !stillOnline {
					h.broadcastPresence(client.entityID, false)
				}
			} else {
				h.mu.Unlock()
			}
		}
	}
}

// DeviceInfo describes a connected device.
type DeviceInfo struct {
	DeviceID   string `json:"device_id"`
	DeviceInfo string `json:"device_info"`
	EntityID   int64  `json:"entity_id"`
}

// GetConnectedDevices returns all active devices for an entity.
func (h *Hub) GetConnectedDevices(entityID int64) []DeviceInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var devices []DeviceInfo
	for client := range h.clients {
		if client.entityID == entityID {
			devices = append(devices, DeviceInfo{
				DeviceID:   client.deviceID,
				DeviceInfo: client.deviceInfo,
				EntityID:   client.entityID,
			})
		}
	}
	return devices
}

// ConnectionCount returns the total number of active WebSocket connections.
func (h *Hub) ConnectionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// isOnlineLocked checks if an entity has active connections. Caller must hold h.mu (read or write).
func (h *Hub) isOnlineLocked(entityID int64) bool {
	for client := range h.clients {
		if client.entityID == entityID {
			return true
		}
	}
	return false
}

// broadcastPresence sends entity.online/entity.offline to all conversations the entity belongs to.
func (h *Hub) broadcastPresence(entityID int64, online bool) {
	eventType := "entity.offline"
	if online {
		eventType = "entity.online"
	}

	msg := WSMessage{
		Type: eventType,
		Data: map[string]interface{}{
			"entity_id": entityID,
			"online":    online,
		},
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	// Get all conversations this entity belongs to
	ctx := context.Background()
	convIDs, err := h.store.GetConversationIDsByEntity(ctx, entityID)
	if err != nil {
		return
	}

	h.mu.RLock()
	sent := make(map[*Client]bool)
	for _, convID := range convIDs {
		for client := range h.convClients[convID] {
			if client.entityID == entityID || sent[client] {
				continue
			}
			select {
			case client.send <- data:
			default:
				log.Printf("ws: entity %d send buffer full (presence), dropping", client.entityID)
			}
			sent[client] = true
		}
	}
	h.mu.RUnlock()
}

func (h *Hub) Register(client *Client) {
	h.register <- client
}

// subscribeClientLocked subscribes a client to its conversations. Caller must hold h.mu write lock.
func (h *Hub) subscribeClientLocked(client *Client) {
	ctx := context.Background()
	ids, err := h.store.GetConversationIDsByEntity(ctx, client.entityID)
	if err != nil {
		log.Printf("ws: failed to get conversations for entity %d: %v", client.entityID, err)
		return
	}

	for _, id := range ids {
		if h.convClients[id] == nil {
			h.convClients[id] = make(map[*Client]bool)
		}
		h.convClients[id][client] = true
	}
}

// unsubscribeClientLocked removes a client from all conversations. Caller must hold h.mu write lock.
func (h *Hub) unsubscribeClientLocked(client *Client) {
	for convID, clients := range h.convClients {
		delete(clients, client)
		if len(clients) == 0 {
			delete(h.convClients, convID)
		}
	}
}

// NotifyNewConversation subscribes connected participants to a new conversation.
func (h *Hub) NotifyNewConversation(convID int64, entityIDs ...int64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.convClients[convID] == nil {
		h.convClients[convID] = make(map[*Client]bool)
	}

	for client := range h.clients {
		for _, eid := range entityIDs {
			if client.entityID == eid {
				h.convClients[convID][client] = true
			}
		}
	}
}

// SendToEntity sends a message to all WebSocket connections of a specific entity.
func (h *Hub) SendToEntity(entityID int64, msg WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	h.mu.RLock()
	// Collect targets under lock
	var targets []*Client
	for client := range h.clients {
		if client.entityID == entityID {
			targets = append(targets, client)
		}
	}
	h.mu.RUnlock()

	// Send outside lock
	for _, client := range targets {
		select {
		case client.send <- data:
		default:
			log.Printf("ws: entity %d send buffer full (direct), dropping", client.entityID)
		}
	}
}

// IsOnline returns true if the entity has at least one active WebSocket connection.
func (h *Hub) IsOnline(entityID int64) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.isOnlineLocked(entityID)
}

// copyConvClients returns a snapshot of clients for a conversation. Caller should NOT hold h.mu.
func (h *Hub) copyConvClients(convID int64) []*Client {
	h.mu.RLock()
	src := h.convClients[convID]
	result := make([]*Client, 0, len(src))
	for client := range src {
		result = append(result, client)
	}
	h.mu.RUnlock()
	return result
}

// BroadcastMessage sends a persisted message to all connected clients in the conversation,
// respecting each participant's subscription mode.
func (h *Hub) BroadcastMessage(msg *model.Message) {
	payload := WSMessage{
		Type: "message.new",
		Data: msg,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	// Build mention set for quick lookup
	mentionSet := make(map[int64]bool, len(msg.Mentions))
	for _, eid := range msg.Mentions {
		mentionSet[eid] = true
	}

	// Load participant subscription modes, entity types, and context windows
	ctx := context.Background()
	participants, err := h.store.ListParticipants(ctx, msg.ConversationID)
	if err != nil {
		log.Printf("ws: failed to load participants for conversation %d: %v", msg.ConversationID, err)
	}
	subModes := make(map[int64]model.SubscriptionMode)
	entityTypes := make(map[int64]model.EntityType)
	contextWindows := make(map[int64]int)
	for _, p := range participants {
		subModes[p.EntityID] = p.SubscriptionMode
		contextWindows[p.EntityID] = p.ContextWindow
		if p.Entity != nil {
			entityTypes[p.EntityID] = p.Entity.EntityType
		}
	}

	// Snapshot clients under lock, iterate without lock
	clients := h.copyConvClients(msg.ConversationID)

	for _, client := range clients {
		// Always deliver to the sender
		if client.entityID == msg.SenderID {
			select {
			case client.send <- data:
			default:
				log.Printf("ws: entity %d send buffer full (broadcast-sender), dropping", client.entityID)
			}
			continue
		}

		// Human users always receive all messages in groups
		if entityTypes[client.entityID] == model.EntityUser {
			select {
			case client.send <- data:
			default:
				log.Printf("ws: entity %d send buffer full (broadcast-user), dropping", client.entityID)
			}
			continue
		}

		// Bots/services: respect subscription mode
		mode := subModes[client.entityID]
		if mode == "" {
			mode = model.SubMentionOnly
		}

		shouldDeliver := false
		switch mode {
		case model.SubSubscribeAll:
			shouldDeliver = true
		case model.SubMentionOnly:
			shouldDeliver = mentionSet[client.entityID]
		case model.SubMentionWithCtx:
			shouldDeliver = mentionSet[client.entityID]
		case model.SubSubscribeDigest:
			shouldDeliver = false // bot polls manually via REST
		}

		if shouldDeliver {
			// For mention_with_context, enrich payload with recent messages
			deliveryData := data
			if mode == model.SubMentionWithCtx {
				ctxWindow := contextWindows[client.entityID]
				if ctxWindow <= 0 {
					ctxWindow = 5
				}
				recentMsgs, err := h.store.ListMessages(ctx, msg.ConversationID, msg.ID, ctxWindow)
				if err == nil && len(recentMsgs) > 0 {
					enriched := WSMessage{
						Type: "message.new",
						Data: map[string]interface{}{
							"message":          msg,
							"context_messages": recentMsgs,
						},
					}
					if enrichedData, err := json.Marshal(enriched); err == nil {
						deliveryData = enrichedData
					}
				}
			}
			select {
			case client.send <- deliveryData:
			default:
				log.Printf("ws: entity %d send buffer full (broadcast-bot), dropping", client.entityID)
			}
		}
	}

	// Notify long-polling waiters for all participants except the sender
	h.notifyParticipantWaiters(msg.ConversationID, msg.SenderID)

	// Send push notifications to offline human users
	if h.OnPush != nil {
		for _, p := range participants {
			if p.EntityID == msg.SenderID {
				continue
			}
			// Only push to human users
			if p.Entity == nil || p.Entity.EntityType != model.EntityUser {
				continue
			}
			if !h.IsOnline(p.EntityID) {
				go h.OnPush(p.EntityID, msg)
			}
		}
	}
}

// BroadcastStream sends an ephemeral stream message (not persisted).
func (h *Hub) BroadcastStream(convID int64, streamType string, payload interface{}, excludeClient *Client) {
	msg := WSMessage{
		Type: "stream." + streamType,
		Data: payload,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	clients := h.copyConvClients(convID)

	for _, client := range clients {
		if client == excludeClient {
			continue
		}
		select {
		case client.send <- data:
		default:
			log.Printf("ws: entity %d send buffer full (stream), dropping", client.entityID)
		}
	}
}

// BroadcastEvent sends a non-stream event to all clients in a conversation.
// Unlike BroadcastStream, it does NOT prepend "stream." to the type.
func (h *Hub) BroadcastEvent(convID int64, eventType string, payload interface{}) {
	msg := WSMessage{
		Type: eventType,
		Data: payload,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	clients := h.copyConvClients(convID)

	for _, client := range clients {
		select {
		case client.send <- data:
		default:
			log.Printf("ws: entity %d send buffer full (event), dropping", client.entityID)
		}
	}
}

func (h *Hub) handleSend(client *Client, rawData []byte) {
	var envelope struct {
		Type string      `json:"type"`
		Data SendPayload `json:"data"`
	}
	if err := json.Unmarshal(rawData, &envelope); err != nil {
		client.sendError("invalid message format")
		return
	}

	payload := envelope.Data

	// Verify participant
	ctx := context.Background()
	ok, err := h.store.IsParticipant(ctx, payload.ConversationID, client.entityID)
	if err != nil || !ok {
		client.sendError("not a participant of this conversation")
		return
	}

	// Handle stream messages (ephemeral)
	if payload.StreamType == "start" || payload.StreamType == "delta" {
		h.BroadcastStream(payload.ConversationID, payload.StreamType, map[string]interface{}{
			"conversation_id": payload.ConversationID,
			"stream_id":       payload.StreamID,
			"sender_id":       client.entityID,
			"layers":          payload.Layers,
		}, client)
		return
	}

	// Persist message (normal send or stream_end)
	contentType := payload.ContentType
	if contentType == "" {
		contentType = model.ContentText
	}

	msg := &model.Message{
		ConversationID: payload.ConversationID,
		SenderID:       client.entityID,
		StreamID:       payload.StreamID,
		ContentType:    contentType,
		Layers:         payload.Layers,
		Attachments:    payload.Attachments,
		Mentions:       payload.Mentions,
		ReplyTo:        payload.ReplyTo,
	}

	if err := h.store.CreateMessage(ctx, msg); err != nil {
		client.sendError("failed to save message")
		return
	}

	_ = h.store.TouchConversation(ctx, payload.ConversationID)

	// Populate sender info before broadcasting
	sender, err := h.store.GetEntityByID(ctx, client.entityID)
	if err == nil {
		msg.SenderType = string(sender.EntityType)
		msg.Sender = sender
	}

	h.BroadcastMessage(msg)
}

// Long polling support

func (h *Hub) RegisterWaiter(entityID int64) chan struct{} {
	h.waiterMu.Lock()
	defer h.waiterMu.Unlock()
	ch := make(chan struct{}, 1)
	h.waiters[entityID] = append(h.waiters[entityID], ch)
	return ch
}

func (h *Hub) UnregisterWaiter(entityID int64, ch chan struct{}) {
	h.waiterMu.Lock()
	defer h.waiterMu.Unlock()
	waiters := h.waiters[entityID]
	for i, w := range waiters {
		if w == ch {
			h.waiters[entityID] = append(waiters[:i], waiters[i+1:]...)
			break
		}
	}
	if len(h.waiters[entityID]) == 0 {
		delete(h.waiters, entityID)
	}
}

// notifyParticipantWaiters notifies long-polling waiters for all participants
// of a conversation except the sender.
func (h *Hub) notifyParticipantWaiters(conversationID, senderID int64) {
	ctx := context.Background()
	participants, err := h.store.ListParticipants(ctx, conversationID)
	if err != nil {
		return
	}

	h.waiterMu.Lock()
	defer h.waiterMu.Unlock()

	for _, p := range participants {
		if p.EntityID == senderID {
			continue
		}
		for _, ch := range h.waiters[p.EntityID] {
			select {
			case ch <- struct{}{}:
			default:
			}
		}
		// Don't nil out — let UnregisterWaiter clean up properly
	}
}

// HandleTaskCancel processes a task cancellation request from a user.
func (h *Hub) handleTaskCancel(client *Client, rawData []byte) {
	var envelope struct {
		Type string `json:"type"`
		Data struct {
			ConversationID int64  `json:"conversation_id"`
			StreamID       string `json:"stream_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rawData, &envelope); err != nil {
		client.sendError("invalid message format")
		return
	}

	payload := envelope.Data
	if payload.StreamID == "" {
		client.sendError("stream_id is required")
		return
	}

	// Broadcast task.cancel to all clients in the conversation (including bots)
	h.BroadcastEvent(payload.ConversationID, "task.cancel", map[string]interface{}{
		"conversation_id": payload.ConversationID,
		"stream_id":       payload.StreamID,
		"cancelled_by":    client.entityID,
	})

	// Confirm cancellation to the sender
	client.sendJSON(WSMessage{
		Type: "task.cancelled",
		Data: map[string]interface{}{
			"conversation_id": payload.ConversationID,
			"stream_id":       payload.StreamID,
		},
	})

	log.Printf("ws: stream %s in conversation %d cancelled by entity %d",
		payload.StreamID, payload.ConversationID, client.entityID)
}
