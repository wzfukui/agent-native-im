package auth

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/wzfukui/agent-native-im/internal/model"
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

// EntityAuth is the unified authentication middleware.
// It first tries JWT (for user sessions), then falls back to API key lookup (for bots/services).
// On success it sets "entityID" (int64) and "entityType" (model.EntityType) in the Gin context.
func EntityAuth(secret string, st store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := extractBearer(c)
		if tokenStr == "" {
			fail(c, http.StatusUnauthorized, "missing authorization")
			c.Abort()
			return
		}

		// 1. Try JWT
		claims, err := ParseToken(secret, tokenStr)
		if err == nil {
			c.Set("entityID", claims.EntityID)
			c.Set("entityType", claims.EntityType)
			c.Next()
			return
		}

		// 2. Try API key: prefix lookup + hash comparison
		if len(tokenStr) >= 8 {
			prefix := tokenStr[:8]
			fullHash := fmt.Sprintf("%x", sha256.Sum256([]byte(tokenStr)))

			creds, err := st.GetCredentialByPrefix(c.Request.Context(), model.CredAPIKey, prefix)
			if err == nil {
				for _, cred := range creds {
					if cred.SecretHash == fullHash {
						entity, err := st.GetEntityByID(c.Request.Context(), cred.EntityID)
						if err == nil {
							c.Set("entityID", entity.ID)
							c.Set("entityType", entity.EntityType)
							c.Next()
							return
						}
					}
				}
			}
		}

		fail(c, http.StatusUnauthorized, "invalid token")
		c.Abort()
	}
}

// GetEntityID extracts the authenticated entity ID from context.
func GetEntityID(c *gin.Context) int64 {
	return c.GetInt64("entityID")
}

// GetEntityType extracts the authenticated entity type from context.
func GetEntityType(c *gin.Context) model.EntityType {
	v, _ := c.Get("entityType")
	if et, ok := v.(model.EntityType); ok {
		return et
	}
	return ""
}
