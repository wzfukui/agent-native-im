package ws

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeTimeout = 10 * time.Second
	pongTimeout  = 60 * time.Second
	pingInterval = 30 * time.Second
	maxMsgSize   = 64 * 1024 // 64KB
)

type Client struct {
	hub        *Hub
	conn       *websocket.Conn
	entityID   int64
	deviceID   string
	deviceInfo string
	send       chan []byte
}

func NewClient(hub *Hub, conn *websocket.Conn, entityID int64, deviceID, deviceInfo string) *Client {
	return &Client{
		hub:        hub,
		conn:       conn,
		entityID:   entityID,
		deviceID:   deviceID,
		deviceInfo: deviceInfo,
		send:       make(chan []byte, 256),
	}
}

func (c *Client) ReadPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMsgSize)
	c.conn.SetReadDeadline(time.Now().Add(pongTimeout))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongTimeout))
		return nil
	})

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("ws: entity %d read error: %v", c.entityID, err)
			}
			return
		}

		var msg WSMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			c.sendError("invalid message format")
			continue
		}

		switch msg.Type {
		case "message.send":
			c.hub.handleSend(c, data)
		case "task.cancel":
			c.hub.handleTaskCancel(c, data)
		case "typing":
			c.hub.handleTyping(c, data)
		case "status.update":
			c.hub.handleStatusUpdate(c, data)
		case "ping":
			c.sendJSON(WSMessage{Type: "pong"})
		default:
			c.sendError("unknown message type: " + msg.Type)
		}
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(pingInterval)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) sendJSON(v interface{}) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("ws: entity %d send recovered from panic (channel closed)", c.entityID)
		}
	}()
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	select {
	case c.send <- data:
	default:
		log.Printf("ws: entity %d send buffer full, dropping message", c.entityID)
	}
}

func (c *Client) sendError(msg string) {
	c.sendJSON(WSMessage{Type: "error", Data: msg})
}
