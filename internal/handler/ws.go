package handler

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	gorillaWs "github.com/gorilla/websocket"
	"github.com/wzfukui/agent-native-im/internal/auth"
	ws_pkg "github.com/wzfukui/agent-native-im/internal/ws"
)

var upgrader = gorillaWs.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // MVP: accept any origin
	},
}

func (s *Server) HandleWS(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		Fail(c, http.StatusUnauthorized, "token required as query parameter")
		return
	}

	var senderType string
	var senderID int64

	// Try JWT first
	claims, err := auth.ParseToken(s.Config.JWTSecret, token)
	if err == nil {
		senderType = "user"
		senderID = claims.UserID
	} else {
		// Try bot token
		bot, err := s.Store.GetBotByToken(c.Request.Context(), token)
		if err != nil {
			Fail(c, http.StatusUnauthorized, "invalid token")
			return
		}
		senderType = "bot"
		senderID = bot.ID
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("ws: upgrade error for %s:%d: %v", senderType, senderID, err)
		return
	}

	client := ws_pkg.NewClient(s.Hub, conn, senderType, senderID)
	s.Hub.Register(client)

	go client.WritePump()
	go client.ReadPump()
}
