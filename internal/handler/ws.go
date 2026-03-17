package handler

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	gorillaWs "github.com/gorilla/websocket"
	"github.com/wzfukui/agent-native-im/internal/auth"
	"github.com/wzfukui/agent-native-im/internal/model"
	"github.com/wzfukui/agent-native-im/internal/utils"
	ws_pkg "github.com/wzfukui/agent-native-im/internal/ws"
)

var upgrader = gorillaWs.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Security: Whitelist allowed origins for WebSocket connections
		origin := r.Header.Get("Origin")
		allowedOrigins := []string{
			"https://ani-web.51pwd.com",
			"http://localhost:3000",     // Development
			"http://localhost:5173",     // Vite dev server
			"http://192.168.44.43:3000", // Local network testing
			"http://127.0.0.1:3000",     // Alternative localhost
			"http://127.0.0.1:5173",     // Alternative Vite
		}

		// Allow requests without origin header (same-origin or non-browser clients)
		if origin == "" {
			return true
		}

		// Check against whitelist
		for _, allowed := range allowedOrigins {
			if origin == allowed {
				return true
			}
		}

		slog.Warn("WebSocket connection rejected", "origin", origin)
		return false
	},
}

func (s *Server) HandleWS(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		// Fallback: read from cookie (browser WebSocket sends cookies automatically)
		if cookie, err := c.Cookie("aim_token"); err == nil && cookie != "" {
			token = cookie
		}
	}
	if token == "" {
		FailWithCode(c, http.StatusUnauthorized, ErrCodeAuthRequired, "token required (via query parameter or cookie)")
		return
	}

	var entityID int64
	var isBootstrap bool

	// Try JWT first
	claims, err := auth.ParseToken(s.Config.JWTSecret, token)
	if err == nil {
		entityID = claims.EntityID
	} else {
		// Try API key
		cred, err := auth.ResolveAPIKey(c.Request.Context(), s.Store, token)
		if err != nil {
			FailWithCode(c, http.StatusUnauthorized, ErrCodeAuthInvalid, "invalid token")
			return
		}
		entityID = cred.EntityID
		isBootstrap = cred.CredType == model.CredBootstrap
	}

	deviceID := c.Query("device_id")
	if deviceID == "" {
		deviceID = fmt.Sprintf("srv-%x", sha256.Sum256([]byte(fmt.Sprintf("%d-%d", entityID, time.Now().UnixNano()))))[:16]
	}
	deviceInfo := c.Query("device_info")

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Error("ws: upgrade error", "entity_id", entityID, "error", err)
		return
	}

	// Parse since_id for message catch-up on reconnect
	var sinceID int64
	if sinceStr := c.Query("since_id"); sinceStr != "" {
		sinceID, _ = strconv.ParseInt(sinceStr, 10, 64)
	}

	client := ws_pkg.NewClient(s.Hub, conn, entityID, deviceID, deviceInfo)
	s.Hub.Register(client)

	// Start WebSocket pumps with panic recovery
	utils.SafeGo(fmt.Sprintf("ws-write-%d", entityID), client.WritePump)
	utils.SafeGo(fmt.Sprintf("ws-read-%d", entityID), client.ReadPump)

	// Send catch-up messages if since_id was provided (reconnect scenario)
	if sinceID > 0 {
		utils.SafeGo(fmt.Sprintf("ws-catchup-%d", entityID), func() {
			s.sendCatchUpMessages(client, entityID, sinceID)
		})
	}

	// Auto-approve if configured globally OR per-entity metadata
	if isBootstrap {
		shouldAutoApprove := s.Config.AutoApproveAgents
		if !shouldAutoApprove {
			// Check per-entity metadata for auto_approve flag
			entity, err := s.Store.GetEntityByID(context.Background(), entityID)
			if err == nil && len(entity.Metadata) > 0 {
				var meta map[string]interface{}
				if json.Unmarshal(entity.Metadata, &meta) == nil {
					if v, ok := meta["auto_approve"]; ok {
						if b, ok := v.(bool); ok && b {
							shouldAutoApprove = true
						}
					}
				}
			}
		}
		if shouldAutoApprove {
			utils.SafeGo(fmt.Sprintf("auto-approve-%d", entityID), func() {
				s.autoApproveEntity(entityID)
			})
		}
	}
}

// sendCatchUpMessages queries missed messages and sends them to the client as message.new events.
func (s *Server) sendCatchUpMessages(client *ws_pkg.Client, entityID, sinceID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	msgs, err := s.Store.GetUpdatesForEntity(ctx, entityID, sinceID)
	if err != nil {
		slog.Error("ws: catch-up query failed", "entity_id", entityID, "since_id", sinceID, "error", err)
		return
	}

	if len(msgs) == 0 {
		return
	}

	slog.Info("ws: sending catch-up messages", "count", len(msgs), "entity_id", entityID, "since_id", sinceID)

	// Populate sender info for each message (batch)
	s.populateSenders(ctx, msgs)

	// Send each message as a message.new event
	for _, msg := range msgs {
		client.SendJSON(ws_pkg.WSMessage{
			Type: "message.new",
			Data: msg,
		})
	}
}

// autoApproveEntity generates a permanent key, deletes bootstrap creds, and pushes via WS.
func (s *Server) autoApproveEntity(entityID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	permanentKey := generateKey(keyPrefixPermanent)
	keyHash := fmt.Sprintf("%x", sha256.Sum256([]byte(permanentKey)))

	cred := &model.Credential{
		EntityID:   entityID,
		CredType:   model.CredAPIKey,
		SecretHash: keyHash,
		RawPrefix:  permanentKey[:8],
	}

	if err := s.Store.CreateCredential(ctx, cred); err != nil {
		slog.Error("auto-approve: failed to create credential", "entity_id", entityID, "error", err)
		return
	}

	if err := s.Store.DeleteCredentialsByType(ctx, entityID, model.CredBootstrap); err != nil {
		slog.Error("auto-approve: failed to delete bootstrap keys", "entity_id", entityID, "error", err)
		return
	}

	s.Hub.SendToEntity(entityID, ws_pkg.WSMessage{
		Type: "connection.approved",
		Data: map[string]interface{}{
			"api_key": permanentKey,
			"message": "Connection auto-approved. Use this permanent key for all future requests.",
		},
	})

	slog.Info("auto-approve: entity approved with permanent key", "entity_id", entityID)
}
