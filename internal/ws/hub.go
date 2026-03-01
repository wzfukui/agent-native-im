package ws

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	"github.com/wzfukui/agent-native-im/internal/model"
	"github.com/wzfukui/agent-native-im/internal/store"
	"github.com/wzfukui/agent-native-im/internal/webhook"
)

type Hub struct {
	store      *store.Store
	webhook    *webhook.Deliverer
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client

	// conversation -> set of clients
	convClients map[int64]map[*Client]bool
	mu          sync.RWMutex

	// Long polling waiters: botID -> channels
	waiters  map[int64][]chan struct{}
	waiterMu sync.Mutex
}

func NewHub(s *store.Store, wh *webhook.Deliverer) *Hub {
	return &Hub{
		store:       s,
		webhook:     wh,
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
			log.Printf("ws: registering %s:%d ...", client.senderType, client.senderID)
			h.subscribeClient(client)
			log.Printf("ws: %s:%d connected (total: %d)", client.senderType, client.senderID, len(h.clients))

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				h.unsubscribeClient(client)
				log.Printf("ws: %s:%d disconnected (total: %d)", client.senderType, client.senderID, len(h.clients))
			}
		}
	}
}

// Register adds a client to the hub.
func (h *Hub) Register(client *Client) {
	h.register <- client
}

func (h *Hub) subscribeClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	var ids []int64
	var err error
	ctx := context.Background()

	if client.senderType == "user" {
		ids, err = h.store.GetConversationIDsByUser(ctx, client.senderID)
	} else {
		ids, err = h.store.GetConversationIDsByBot(ctx, client.senderID)
	}
	if err != nil {
		log.Printf("ws: failed to get conversations for %s:%d: %v", client.senderType, client.senderID, err)
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

// NotifyNewConversation adds connected clients to a new conversation's subscription.
func (h *Hub) NotifyNewConversation(convID, userID, botID int64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.convClients[convID] == nil {
		h.convClients[convID] = make(map[*Client]bool)
	}

	for client := range h.clients {
		if (client.senderType == "user" && client.senderID == userID) ||
			(client.senderType == "bot" && client.senderID == botID) {
			h.convClients[convID][client] = true
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
			// buffer full, skip
		}
	}

	// Notify long-polling waiters + webhook
	if msg.SenderType == "user" {
		conv, err := h.store.GetConversation(context.Background(), msg.ConversationID)
		if err == nil {
			h.notifyWaiters(conv.BotID)
		}
		// Deliver webhook asynchronously
		if h.webhook != nil {
			h.webhook.DeliverAsync(msg)
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

	// Handle stream messages
	if payload.StreamType == "start" || payload.StreamType == "delta" {
		// Ephemeral: broadcast but don't persist
		h.BroadcastStream(payload.ConversationID, payload.StreamType, map[string]interface{}{
			"conversation_id": payload.ConversationID,
			"stream_id":       payload.StreamID,
			"sender_type":     client.senderType,
			"sender_id":       client.senderID,
			"layers":          payload.Layers,
		}, client)
		return
	}

	// Persist message (normal send or stream_end)
	msg := &model.Message{
		ConversationID: payload.ConversationID,
		StreamID:       payload.StreamID,
		SenderType:     client.senderType,
		SenderID:       client.senderID,
		Layers:         payload.Layers,
	}

	if err := h.store.CreateMessage(context.Background(), msg); err != nil {
		client.sendError("failed to save message")
		return
	}

	_ = h.store.TouchConversation(context.Background(), payload.ConversationID)

	h.BroadcastMessage(msg)
}

// Long polling support

func (h *Hub) RegisterWaiter(botID int64) chan struct{} {
	h.waiterMu.Lock()
	defer h.waiterMu.Unlock()
	ch := make(chan struct{}, 1)
	h.waiters[botID] = append(h.waiters[botID], ch)
	return ch
}

func (h *Hub) UnregisterWaiter(botID int64, ch chan struct{}) {
	h.waiterMu.Lock()
	defer h.waiterMu.Unlock()
	waiters := h.waiters[botID]
	for i, w := range waiters {
		if w == ch {
			h.waiters[botID] = append(waiters[:i], waiters[i+1:]...)
			break
		}
	}
}

func (h *Hub) notifyWaiters(botID int64) {
	h.waiterMu.Lock()
	defer h.waiterMu.Unlock()
	for _, ch := range h.waiters[botID] {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
	h.waiters[botID] = nil
}
