package handler

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		if isAllowedBrowserOrigin(origin) {
			return true
		}

		slog.Warn("WebSocket connection rejected", "origin", origin)
		return false
	},
}

const wsBearerSubprotocolPrefix = "ani.bearer."

func isAllowedBrowserOrigin(origin string) bool {
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}

	switch parsed.Scheme {
	case "https":
		return true
	case "http":
		host := parsed.Hostname()
		return host == "localhost" || host == "127.0.0.1" || strings.HasPrefix(host, "192.168.")
	default:
		return false
	}
}

func extractWebSocketToken(r *http.Request) (token string, selectedSubprotocol string) {
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return authHeader[7:], ""
	}

	for _, protocol := range gorillaWs.Subprotocols(r) {
		if strings.HasPrefix(protocol, wsBearerSubprotocolPrefix) {
			return strings.TrimPrefix(protocol, wsBearerSubprotocolPrefix), protocol
		}
	}

	if cookie, err := r.Cookie("aim_token"); err == nil && cookie.Value != "" {
		return cookie.Value, ""
	}

	return "", ""
}

func (s *Server) HandleWS(c *gin.Context) {
	token, selectedSubprotocol := extractWebSocketToken(c.Request)
	if token == "" {
		FailWithCode(c, http.StatusUnauthorized, ErrCodeAuthRequired, "token required (via Authorization header, WebSocket subprotocol, or cookie)")
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

	// Reject disabled entities before upgrading the connection
	entity, err := s.Store.GetEntityByID(c.Request.Context(), entityID)
	if err != nil {
		slog.Error("ws: failed to look up entity", "entity_id", entityID, "error", err)
		FailWithCode(c, http.StatusUnauthorized, ErrCodeEntityNotFound, "entity not found")
		return
	}
	if entity.Status == "disabled" {
		FailWithCode(c, http.StatusForbidden, ErrCodePermDenied, "entity is disabled")
		return
	}

	deviceID := c.Query("device_id")
	if deviceID == "" {
		deviceID = fmt.Sprintf("srv-%x", sha256.Sum256([]byte(fmt.Sprintf("%d-%d", entityID, time.Now().UnixNano()))))[:16]
	}
	deviceInfo := c.Query("device_info")

	wsUpgrader := upgrader
	if selectedSubprotocol != "" {
		wsUpgrader.Subprotocols = []string{selectedSubprotocol}
	}

	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
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
