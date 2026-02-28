package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/wzfukui/agent-native-im/internal/store"
)

func extractBearer(c *gin.Context) string {
	h := c.GetHeader("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return h[7:]
	}
	return ""
}

func fail(c *gin.Context, code int, msg string) {
	c.JSON(code, gin.H{"ok": false, "error": msg})
}

func UserAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := extractBearer(c)
		if tokenStr == "" {
			fail(c, http.StatusUnauthorized, "missing authorization")
			c.Abort()
			return
		}
		claims, err := ParseToken(secret, tokenStr)
		if err != nil {
			fail(c, http.StatusUnauthorized, "invalid token")
			c.Abort()
			return
		}
		c.Set("userID", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("senderType", "user")
		c.Set("senderID", claims.UserID)
		c.Next()
	}
}

func BotAuth(s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := extractBearer(c)
		if tokenStr == "" {
			fail(c, http.StatusUnauthorized, "missing authorization")
			c.Abort()
			return
		}
		bot, err := s.GetBotByToken(c.Request.Context(), tokenStr)
		if err != nil {
			fail(c, http.StatusUnauthorized, "invalid bot token")
			c.Abort()
			return
		}
		c.Set("botID", bot.ID)
		c.Set("ownerID", bot.OwnerID)
		c.Set("senderType", "bot")
		c.Set("senderID", bot.ID)
		c.Next()
	}
}

func AnyAuth(secret string, s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := extractBearer(c)
		if tokenStr == "" {
			fail(c, http.StatusUnauthorized, "missing authorization")
			c.Abort()
			return
		}

		// Try JWT first
		claims, err := ParseToken(secret, tokenStr)
		if err == nil {
			c.Set("userID", claims.UserID)
			c.Set("username", claims.Username)
			c.Set("senderType", "user")
			c.Set("senderID", claims.UserID)
			c.Next()
			return
		}

		// Try bot token
		bot, err := s.GetBotByToken(c.Request.Context(), tokenStr)
		if err != nil {
			fail(c, http.StatusUnauthorized, "invalid token")
			c.Abort()
			return
		}
		c.Set("botID", bot.ID)
		c.Set("ownerID", bot.OwnerID)
		c.Set("senderType", "bot")
		c.Set("senderID", bot.ID)
		c.Next()
	}
}
