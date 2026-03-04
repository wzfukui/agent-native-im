package handler

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"net/http"
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

		log.Printf("WebSocket connection rejected from origin: %s", origin)
		return false
	},
}

func (s *Server) HandleWS(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		FailWithCode(c, http.StatusUnauthorized, ErrCodeAuthRequired, "token required as query parameter")
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
		log.Printf("ws: upgrade error for entity %d: %v", entityID, err)
		return
	}

	client := ws_pkg.NewClient(s.Hub, conn, entityID, deviceID, deviceInfo)
	s.Hub.Register(client)

	// Start WebSocket pumps with panic recovery
	utils.SafeGo(fmt.Sprintf("ws-write-%d", entityID), client.WritePump)
	utils.SafeGo(fmt.Sprintf("ws-read-%d", entityID), client.ReadPump)

	// Auto-approve if configured and the agent connected with a bootstrap key
	if isBootstrap && s.Config.AutoApproveAgents {
		utils.SafeGo(fmt.Sprintf("auto-approve-%d", entityID), func() {
			s.autoApproveEntity(entityID)
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
		log.Printf("auto-approve: failed to create credential for entity %d: %v", entityID, err)
		return
	}

	if err := s.Store.DeleteCredentialsByType(ctx, entityID, model.CredBootstrap); err != nil {
		log.Printf("auto-approve: failed to delete bootstrap keys for entity %d: %v", entityID, err)
		return
	}

	s.Hub.SendToEntity(entityID, ws_pkg.WSMessage{
		Type: "connection.approved",
		Data: map[string]interface{}{
			"api_key": permanentKey,
			"message": "Connection auto-approved. Use this permanent key for all future requests.",
		},
	})

	log.Printf("auto-approve: entity %d approved with permanent key", entityID)
}
