package auth

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wzfukui/agent-native-im/internal/model"
	"github.com/wzfukui/agent-native-im/internal/store"
)

func extractBearer(c *gin.Context) string {
	// 1. Authorization header (highest priority — API keys, SDK, programmatic access)
	h := c.GetHeader("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return h[7:]
	}

	// 2. HttpOnly cookie (web browser sessions)
	if cookie, err := c.Cookie("aim_token"); err == nil && cookie != "" {
		return cookie
	}

	// 3. Query parameter (deprecated; allowed only for file downloads)
	if strings.HasPrefix(c.Request.URL.Path, "/files/") {
		if t := c.Query("token"); t != "" {
			slog.Warn("auth: token passed via file query parameter (deprecated)", "path", c.Request.URL.Path)
			return t
		}
	}
	return ""
}

func fail(c *gin.Context, code int, msg string) {
	c.JSON(code, gin.H{"ok": false, "error": msg})
}

// EntityAuth is the unified authentication middleware.
// It first tries JWT (for user sessions), then falls back to API key lookup (for bots/services).
// On success it sets "entityID" (int64) and "entityType" (model.EntityType) in the Gin context.
// If authenticated via bootstrap key, it also sets "bootstrapPending" (bool) = true.
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
			entity, getErr := st.GetEntityByID(c.Request.Context(), claims.EntityID)
			if getErr != nil {
				fail(c, http.StatusUnauthorized, "invalid token")
				c.Abort()
				return
			}
			if entity.Status == "disabled" {
				fail(c, http.StatusForbidden, "entity is disabled")
				c.Abort()
				return
			}
			c.Set("entityID", entity.ID)
			c.Set("entityType", entity.EntityType)
			c.Next()
			return
		}

		// 1.1 Allow recently-expired JWTs only for token refresh endpoint.
		// This enables automatic session recovery after short offline periods.
		if c.Request.URL.Path == "/api/v1/auth/refresh" {
			expiredClaims, expiredErr := ParseTokenAllowExpired(secret, tokenStr)
			if expiredErr == nil && expiredClaims.ExpiresAt != nil {
				now := time.Now()
				expAt := expiredClaims.ExpiresAt.Time
				if !expAt.IsZero() && !expAt.After(now) && now.Sub(expAt) <= 7*24*time.Hour {
					entity, getErr := st.GetEntityByID(c.Request.Context(), expiredClaims.EntityID)
					if getErr == nil {
						if entity.Status == "disabled" {
							fail(c, http.StatusForbidden, "entity is disabled")
							c.Abort()
							return
						}
						c.Set("entityID", entity.ID)
						c.Set("entityType", entity.EntityType)
						c.Set("allowExpiredJWT", true)
						c.Next()
						return
					}
				}
			}
		}

		// 2. Try API key / bootstrap key
		cred, err := ResolveAPIKey(c.Request.Context(), st, tokenStr)
		if err == nil {
			entity, err := st.GetEntityByID(c.Request.Context(), cred.EntityID)
			if err == nil {
				// Check if entity is disabled
				if entity.Status == "disabled" {
					fail(c, http.StatusForbidden, "entity is disabled")
					c.Abort()
					return
				}
				c.Set("entityID", entity.ID)
				c.Set("entityType", entity.EntityType)
				if cred.CredType == model.CredBootstrap {
					c.Set("bootstrapPending", true)
				}
				c.Next()
				return
			}
		}

		fail(c, http.StatusUnauthorized, "invalid token")
		c.Abort()
	}
}

// RequireFullAuth blocks requests authenticated with bootstrap keys.
// Bootstrap keys are only allowed to access /me and /ws.
func RequireFullAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if IsBootstrap(c) {
			fail(c, http.StatusForbidden, "bootstrap key cannot access this endpoint. Bootstrap keys can only access /me and /ws. To get a permanent key: 1) connect via WebSocket (GET /api/v1/ws?token=YOUR_BOOTSTRAP_KEY), 2) wait for auto-approve or manual approval, 3) receive permanent key (aim_ prefix) via WebSocket message. See /api/v1/onboarding-guide for details.")
			c.Abort()
			return
		}
		c.Next()
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

// IsBootstrap returns true if the current request was authenticated with a bootstrap key.
func IsBootstrap(c *gin.Context) bool {
	v, exists := c.Get("bootstrapPending")
	if !exists {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

// RequireAdmin blocks requests from non-admin users.
// Admin is determined by matching the entity name against the configured admin username.
func RequireAdmin(st store.Store, adminUser string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if GetEntityType(c) != model.EntityUser {
			fail(c, http.StatusForbidden, "admin access required")
			c.Abort()
			return
		}
		entity, err := st.GetEntityByID(c.Request.Context(), GetEntityID(c))
		if err != nil || entity.Name != adminUser {
			fail(c, http.StatusForbidden, "admin access required")
			c.Abort()
			return
		}
		c.Next()
	}
}

// SetAuthCookie sets the aim_token HttpOnly cookie on the response.
// In production (non-localhost), Secure is true (HTTPS only).
// SameSite=Lax provides CSRF protection while allowing normal navigation.
func SetAuthCookie(c *gin.Context, token string) {
	secure := true
	host := c.Request.Host
	if strings.Contains(host, "localhost") || strings.Contains(host, "127.0.0.1") {
		secure = false
	}
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("aim_token", token, 7*24*60*60, "/", "", secure, true)
}

// ClearAuthCookie removes the aim_token cookie from the response.
func ClearAuthCookie(c *gin.Context) {
	secure := true
	host := c.Request.Host
	if strings.Contains(host, "localhost") || strings.Contains(host, "127.0.0.1") {
		secure = false
	}
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("aim_token", "", -1, "/", "", secure, true)
}

// ResolveAPIKey looks up an entity by API key or bootstrap key (prefix + hash verification).
// Returns the matching credential on success.
func ResolveAPIKey(ctx context.Context, st store.Store, apiKey string) (*model.Credential, error) {
	if len(apiKey) < 8 {
		return nil, fmt.Errorf("api key too short")
	}
	prefix := apiKey[:8]
	fullHash := fmt.Sprintf("%x", sha256.Sum256([]byte(apiKey)))

	// Determine credential type from key prefix
	credType := model.CredAPIKey
	if strings.HasPrefix(apiKey, "aimb_") {
		credType = model.CredBootstrap
	}

	creds, err := st.GetCredentialByPrefix(ctx, credType, prefix)
	if err != nil {
		return nil, err
	}

	for _, cred := range creds {
		if cred.SecretHash == fullHash {
			return cred, nil
		}
	}

	// If not found with detected type, try the other type as fallback (backward compat)
	if credType == model.CredAPIKey {
		creds, err = st.GetCredentialByPrefix(ctx, model.CredBootstrap, prefix)
	} else {
		creds, err = st.GetCredentialByPrefix(ctx, model.CredAPIKey, prefix)
	}
	if err != nil {
		return nil, err
	}
	for _, cred := range creds {
		if cred.SecretHash == fullHash {
			return cred, nil
		}
	}

	return nil, fmt.Errorf("api key not found")
}
