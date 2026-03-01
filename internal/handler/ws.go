package handler

import (
	"crypto/sha256"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	gorillaWs "github.com/gorilla/websocket"
	"github.com/wzfukui/agent-native-im/internal/auth"
	"github.com/wzfukui/agent-native-im/internal/model"
	ws_pkg "github.com/wzfukui/agent-native-im/internal/ws"
)

var upgrader = gorillaWs.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (s *Server) HandleWS(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		Fail(c, http.StatusUnauthorized, "token required as query parameter")
		return
	}

	var entityID int64

	// Try JWT first
	claims, err := auth.ParseToken(s.Config.JWTSecret, token)
	if err == nil {
		entityID = claims.EntityID
	} else {
		// Try API key
		if len(token) >= 8 {
			eid, err := s.resolveAPIKey(c, token)
			if err != nil {
				Fail(c, http.StatusUnauthorized, "invalid token")
				return
			}
			entityID = eid
		} else {
			Fail(c, http.StatusUnauthorized, "invalid token")
			return
		}
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("ws: upgrade error for entity %d: %v", entityID, err)
		return
	}

	client := ws_pkg.NewClient(s.Hub, conn, entityID)
	s.Hub.Register(client)

	go client.WritePump()
	go client.ReadPump()
}

// resolveAPIKey looks up an entity by API key. Returns entity ID on success.
func (s *Server) resolveAPIKey(c *gin.Context, apiKey string) (int64, error) {
	prefix := apiKey[:8]
	fullHash := fmt.Sprintf("%x", sha256.Sum256([]byte(apiKey)))

	creds, err := s.Store.GetCredentialByPrefix(c.Request.Context(), model.CredAPIKey, prefix)
	if err != nil {
		return 0, err
	}

	for _, cred := range creds {
		if cred.SecretHash == fullHash {
			return cred.EntityID, nil
		}
	}

	return 0, fmt.Errorf("api key not found")
}
