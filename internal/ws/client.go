package ws

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/coder/websocket"
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
	senderType string // "user" or "bot"
	senderID   int64
	send       chan []byte
	ctx        context.Context
	cancel     context.CancelFunc
}

func NewClient(hub *Hub, conn *websocket.Conn, senderType string, senderID int64) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		hub:        hub,
		conn:       conn,
		senderType: senderType,
		senderID:   senderID,
		send:       make(chan []byte, 256),
		ctx:        ctx,
		cancel:     cancel,
	}
}

func (c *Client) ReadPump() {
	defer func() {
		c.hub.unregister <- c
		c.cancel()
		c.conn.Close(websocket.StatusNormalClosure, "")
	}()

	c.conn.SetReadLimit(maxMsgSize)

	for {
		_, data, err := c.conn.Read(c.ctx)
		if err != nil {
			if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
				log.Printf("ws: client %s:%d disconnected normally", c.senderType, c.senderID)
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
		c.conn.Close(websocket.StatusNormalClosure, "")
	}()

	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				return
			}
			ctx, cancel := context.WithTimeout(c.ctx, writeTimeout)
			err := c.conn.Write(ctx, websocket.MessageText, msg)
			cancel()
			if err != nil {
				return
			}

		case <-ticker.C:
			ctx, cancel := context.WithTimeout(c.ctx, writeTimeout)
			err := c.conn.Ping(ctx)
			cancel()
			if err != nil {
				return
			}

		case <-c.ctx.Done():
			return
		}
	}
}

func (c *Client) sendJSON(v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	select {
	case c.send <- data:
	default:
		// buffer full
	}
}

func (c *Client) sendError(msg string) {
	c.sendJSON(WSMessage{Type: "error", Data: msg})
}
