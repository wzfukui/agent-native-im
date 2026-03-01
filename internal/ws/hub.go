package ws

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	"github.com/wzfukui/agent-native-im/internal/model"
	"github.com/wzfukui/agent-native-im/internal/store"
)

type Hub struct {
	store      store.Store
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client

	// conversation -> set of clients
	convClients map[int64]map[*Client]bool
	mu          sync.RWMutex

	// Long polling waiters: entityID -> channels
	waiters  map[int64][]chan struct{}
	waiterMu sync.Mutex
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
			h.clients[client] = true
			h.subscribeClient(client)
			log.Printf("ws: entity %d connected (total: %d)", client.entityID, len(h.clients))

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				h.unsubscribeClient(client)
				log.Printf("ws: entity %d disconnected (total: %d)", client.entityID, len(h.clients))
			}
		}
	}
}

func (h *Hub) Register(client *Client) {
	h.register <- client
}

func (h *Hub) subscribeClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

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

func (h *Hub) unsubscribeClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

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

// BroadcastMessage sends a persisted message to all connected clients in the conversation.
func (h *Hub) BroadcastMessage(msg *model.Message) {
	payload := WSMessage{
		Type: "message.new",
		Data: msg,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	h.mu.RLock()
	clients := h.convClients[msg.ConversationID]
	h.mu.RUnlock()

	for client := range clients {
		select {
		case client.send <- data:
		default:
		}
	}

	// Notify long-polling waiters for all participants except the sender
	h.notifyParticipantWaiters(msg.ConversationID, msg.SenderID)
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

	h.mu.RLock()
	clients := h.convClients[convID]
	h.mu.RUnlock()

	for client := range clients {
		if client == excludeClient {
			continue
		}
		select {
		case client.send <- data:
		default:
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
	}

	if err := h.store.CreateMessage(ctx, msg); err != nil {
		client.sendError("failed to save message")
		return
	}

	_ = h.store.TouchConversation(ctx, payload.ConversationID)

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
		h.waiters[p.EntityID] = nil
	}
}
