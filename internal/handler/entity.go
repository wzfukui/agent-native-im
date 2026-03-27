package handler

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/wzfukui/agent-native-im/internal/auth"
	"github.com/wzfukui/agent-native-im/internal/model"
	"github.com/wzfukui/agent-native-im/internal/ws"
)

const (
	keyPrefixBootstrap = "aimb_"
	keyPrefixPermanent = "aim_"
)

type createEntityRequest struct {
	Name        string                 `json:"name" binding:"required"`
	DisplayName string                 `json:"display_name"`
	EntityType  string                 `json:"entity_type"` // defaults to "bot"
	Metadata    map[string]interface{} `json:"metadata"`
}

// generateKey creates a random API key with the given prefix.
// Format: prefix + 48 hex chars (24 random bytes).
func generateKey(prefix string) string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

// HandleCreateEntity creates a bot/service entity owned by the authenticated user.
// Issues a permanent API key (aim_ prefix) immediately — no bootstrap/approval flow needed.
func (s *Server) HandleCreateEntity(c *gin.Context) {
	if auth.GetEntityType(c) != model.EntityUser {
		FailWithCode(c, http.StatusForbidden, ErrCodePermDenied, "only users can create entities")
		return
	}

	var req createEntityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		FailWithCode(c, http.StatusBadRequest, ErrCodeValidationField, "name is required")
		return
	}

	entityType := model.EntityBot
	if req.EntityType == "service" {
		entityType = model.EntityService
	}

	ownerID := auth.GetEntityID(c)
	displayName := req.DisplayName
	if displayName == "" {
		displayName = req.Name
	}

	entity := &model.Entity{
		EntityType:  entityType,
		Name:        req.Name,
		DisplayName: displayName,
		Status:      "active",
		OwnerID:     &ownerID,
	}

	if req.Metadata != nil {
		metaJSON, err := json.Marshal(req.Metadata)
		if err != nil {
			FailWithCode(c, http.StatusBadRequest, "VALIDATION_METADATA_INVALID",
				"Invalid metadata format")
			return
		}
		// Limit metadata size to 10KB
		const maxMetadataSize = 10 * 1024
		if len(metaJSON) > maxMetadataSize {
			FailWithCode(c, http.StatusBadRequest, "VALIDATION_METADATA_TOO_LARGE",
				fmt.Sprintf("Metadata size exceeds limit (%d bytes > %d bytes)", len(metaJSON), maxMetadataSize))
			return
		}
		entity.Metadata = metaJSON
	}

	if err := s.Store.CreateEntity(c.Request.Context(), entity); err != nil {
		FailFromDB(c, err, "failed to create entity")
		return
	}
	s.attachEntityPublicID(c.Request.Context(), entity)

	// Always issue a permanent API key on creation (like Telegram/Discord/Slack).
	returnedKey := generateKey(keyPrefixPermanent)
	credType := model.CredAPIKey

	keyHash := fmt.Sprintf("%x", sha256.Sum256([]byte(returnedKey)))
	cred := &model.Credential{
		EntityID:   entity.ID,
		CredType:   credType,
		SecretHash: keyHash,
		RawPrefix:  returnedKey[:8],
	}

	if err := s.Store.CreateCredential(c.Request.Context(), cred); err != nil {
		FailFromDB(c, err, "failed to create credential")
		return
	}

	// Build markdown onboarding doc (SKILL format)
	// Derive server URL dynamically from request headers
	serverURL := s.Config.ServerURL // fallback
	if origin := c.GetHeader("Origin"); origin != "" {
		serverURL = origin
	} else if (serverURL == "" || serverURL == "http://localhost:9800") && c.GetHeader("X-Forwarded-Proto") != "" {
		fwdProto := c.GetHeader("X-Forwarded-Proto")
		host := c.GetHeader("X-Forwarded-Host")
		if host == "" {
			host = c.Request.Host
		}
		serverURL = fwdProto + "://" + host
	} else if c.Request.Host != "" && (serverURL == "" || serverURL == "http://localhost:9800") {
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		serverURL = scheme + "://" + c.Request.Host
	}
	wsURL := strings.Replace(strings.Replace(serverURL, "https://", "wss://", 1), "http://", "ws://", 1)

	keyLabel := "API Key"
	keyNote := `- **此密钥为永久密钥（aim_ 前缀），可直接调用所有 API**
- 请妥善保管，创建后不再展示`

	markdownDoc := fmt.Sprintf(`# OpenClaw Access Pack — %s

## Connection Credentials

| 项目 | 值 |
|------|---|
| API Base | `+"`%s/api/v1`"+` |
| WebSocket | `+"`%s/api/v1/ws`"+` |
| `+keyLabel+` | `+"`%s`"+` |
| Entity ID | `+"`%d`"+` |

## Important

`+keyNote+`
- This access pack is intended for the ANI OpenClaw channel plugin.
- Do not wire this token into the Python SDK quickstart. The recommended path is OpenClaw channel mode.

**OpenClaw onboarding guide:** %s/api/v1/onboarding-guide
**LLM output format guide:** %s/api/v1/skill-template?format=text

## Environment Variables

`+"```bash"+`
AGENT_IM_BASE=%s/api/v1
AGENT_IM_TOKEN=%s
AGENT_IM_WS=%s/api/v1/ws
AGENT_IM_ENTITY_ID=%d
`+"```"+`

## Quick Start (OpenClaw Plugin)

`+"```bash"+`
openclaw plugin install ani-openclaw-plugin

# Trust and enable the ANI plugin
openclaw config set plugins.allow '["ani"]' --strict-json
openclaw config set plugins.entries.ani.enabled true

# Configure ANI channel
openclaw config set channels.ani.serverUrl "%s"
openclaw config set channels.ani.apiKey "%s"

# Minimum ANI tool access
openclaw config set tools.profile messaging
openclaw config set tools.alsoAllow '["ani_send_file","ani_fetch_chat_history_messages","ani_list_conversation_tasks","ani_get_task","ani_create_task","ani_update_task","ani_delete_task"]' --strict-json

# Optional: allow public web lookups
openclaw config set tools.allow '["group:web"]' --strict-json

# Start the gateway
openclaw gateway run
`+"```"+`

### Source Install (fallback)

`+"```bash"+`
git clone https://github.com/wzfukui/openclaw.git
cd openclaw
git checkout main
pnpm install
`+"```"+`
`,
		entity.DisplayName,
		serverURL, wsURL, returnedKey, entity.ID,
		serverURL, serverURL,
		serverURL, returnedKey, wsURL, entity.ID,
		serverURL,
		returnedKey,
	)

	OK(c, http.StatusCreated, gin.H{
		"entity":       entity,
		"api_key":      returnedKey,
		"markdown_doc": markdownDoc,
	})
}

// HandleApproveConnection approves a Bot's connection request.
// It generates a permanent API key, deletes the bootstrap key, and pushes the new key via WebSocket.
func (s *Server) HandleApproveConnection(c *gin.Context) {
	if auth.GetEntityType(c) != model.EntityUser {
		FailWithCode(c, http.StatusForbidden, ErrCodePermDenied, "only users can approve connections")
		return
	}

	entityID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid entity id")
		return
	}

	// Verify ownership
	target, err := s.Store.GetEntityByID(c.Request.Context(), entityID)
	if err != nil {
		FailWithCode(c, http.StatusNotFound, ErrCodeEntityNotFound, "entity not found")
		return
	}

	if target.OwnerID == nil || *target.OwnerID != auth.GetEntityID(c) {
		FailWithCode(c, http.StatusForbidden, ErrCodePermNotOwner, "not the owner of this entity")
		return
	}

	// Generate permanent API key
	permanentKey := generateKey(keyPrefixPermanent)
	keyHash := fmt.Sprintf("%x", sha256.Sum256([]byte(permanentKey)))

	cred := &model.Credential{
		EntityID:   entityID,
		CredType:   model.CredAPIKey,
		SecretHash: keyHash,
		RawPrefix:  permanentKey[:8],
	}

	if err := s.Store.CreateCredential(c.Request.Context(), cred); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to create permanent credential")
		return
	}

	// Delete all bootstrap credentials for this entity
	if err := s.Store.DeleteCredentialsByType(c.Request.Context(), entityID, model.CredBootstrap); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to revoke bootstrap key")
		return
	}

	// Push new key to the Bot via WebSocket
	if s.Hub != nil {
		s.Hub.SendToEntity(entityID, ws.WSMessage{
			Type: "connection.approved",
			Data: gin.H{
				"api_key": permanentKey,
				"message": "Connection approved. Use this permanent key for all future requests.",
			},
		})
	}

	OK(c, http.StatusOK, gin.H{
		"message": "connection approved",
		"entity":  target,
	})
}

// HandleEntityStatus returns the online status of an entity.
func (s *Server) HandleEntityStatus(c *gin.Context) {
	entity, entityID, ok := s.ensureOwnedEntity(c)
	if !ok {
		return
	}
	resp := gin.H{
		"entity_id": entity.ID,
		"name":      entity.DisplayName,
		"online":    s.Hub.IsOnline(entityID),
	}
	if lastSeen, ok := s.Hub.LastSeen(entityID); ok {
		resp["last_seen"] = lastSeen
	}
	OK(c, http.StatusOK, resp)
}

// HandleGetCredentials returns credential status for a bot/service entity.
func (s *Server) HandleGetCredentials(c *gin.Context) {
	if auth.GetEntityType(c) != model.EntityUser {
		FailWithCode(c, http.StatusForbidden, ErrCodePermDenied, "only users can view credentials")
		return
	}

	entityID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid entity id")
		return
	}

	ctx := c.Request.Context()
	target, err := s.Store.GetEntityByID(ctx, entityID)
	if err != nil {
		FailWithCode(c, http.StatusNotFound, ErrCodeEntityNotFound, "entity not found")
		return
	}

	if target.OwnerID == nil || *target.OwnerID != auth.GetEntityID(c) {
		FailWithCode(c, http.StatusForbidden, ErrCodePermNotOwner, "not the owner of this entity")
		return
	}

	bootstrapCreds, _ := s.Store.GetCredentialsByEntity(ctx, entityID, model.CredBootstrap)
	apiKeyCreds, _ := s.Store.GetCredentialsByEntity(ctx, entityID, model.CredAPIKey)

	bootstrapPrefix := ""
	if len(bootstrapCreds) > 0 {
		bootstrapPrefix = bootstrapCreds[0].RawPrefix
	}

	OK(c, http.StatusOK, gin.H{
		"entity_id":        entityID,
		"has_bootstrap":    len(bootstrapCreds) > 0,
		"has_api_key":      len(apiKeyCreds) > 0,
		"bootstrap_prefix": bootstrapPrefix,
	})
}

func (s *Server) ensureOwnedEntity(c *gin.Context) (*model.Entity, int64, bool) {
	if auth.GetEntityType(c) != model.EntityUser {
		FailWithCode(c, http.StatusForbidden, ErrCodePermDenied, "only users can manage entities")
		return nil, 0, false
	}

	entityID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid entity id")
		return nil, 0, false
	}

	target, err := s.Store.GetEntityByID(c.Request.Context(), entityID)
	if err != nil {
		FailWithCode(c, http.StatusNotFound, ErrCodeEntityNotFound, "entity not found")
		return nil, 0, false
	}

	if target.OwnerID == nil || *target.OwnerID != auth.GetEntityID(c) {
		FailWithCode(c, http.StatusForbidden, ErrCodePermNotOwner, "not the owner of this entity")
		return nil, 0, false
	}

	return target, entityID, true
}

func (s *Server) issuePermanentCredential(ctx context.Context, entityID int64) (string, string, error) {
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		permanentKey := generateKey(keyPrefixPermanent)
		keyHash := fmt.Sprintf("%x", sha256.Sum256([]byte(permanentKey)))

		cred := &model.Credential{
			EntityID:   entityID,
			CredType:   model.CredAPIKey,
			SecretHash: keyHash,
			RawPrefix:  permanentKey[:8],
		}
		if err := s.Store.CreateCredential(ctx, cred); err != nil {
			lastErr = err
			if isUniqueConstraintError(err) {
				slog.Warn("credential collision during token regeneration", "entity_id", entityID, "attempt", attempt+1, "error", err)
				continue
			}
			return "", "", err
		}
		return permanentKey, keyHash, nil
	}
	return "", "", lastErr
}

// HandleEntitySelfCheck returns a lightweight readiness report for a bot.
func (s *Server) HandleEntitySelfCheck(c *gin.Context) {
	target, entityID, ok := s.ensureOwnedEntity(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()
	bootstrapCreds, _ := s.Store.GetCredentialsByEntity(ctx, entityID, model.CredBootstrap)
	apiKeyCreds, _ := s.Store.GetCredentialsByEntity(ctx, entityID, model.CredAPIKey)

	isOnline := s.Hub.IsOnline(entityID)
	ready := target.Status == "active" && len(apiKeyCreds) > 0

	recommendations := make([]string, 0, 3)
	if target.Status != "active" {
		recommendations = append(recommendations, "entity is disabled, reactivate it first")
	}
	if len(apiKeyCreds) == 0 {
		if len(bootstrapCreds) > 0 {
			recommendations = append(recommendations, "bot is still using bootstrap key, complete approval to issue permanent key")
		} else {
			recommendations = append(recommendations, "no credentials found, recreate or re-approve this bot")
		}
	}
	if !isOnline {
		recommendations = append(recommendations, "bot is offline, verify network and websocket handshake")
	}

	OK(c, http.StatusOK, gin.H{
		"entity_id":      entityID,
		"entity_name":    target.DisplayName,
		"status":         target.Status,
		"online":         isOnline,
		"ready":          ready,
		"has_bootstrap":  len(bootstrapCreds) > 0,
		"has_api_key":    len(apiKeyCreds) > 0,
		"recommendation": recommendations,
	})
}

// HandleEntityDiagnostics returns connection diagnostics for one owned entity.
func (s *Server) HandleEntityDiagnostics(c *gin.Context) {
	target, entityID, ok := s.ensureOwnedEntity(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()
	bootstrapCreds, _ := s.Store.GetCredentialsByEntity(ctx, entityID, model.CredBootstrap)
	apiKeyCreds, _ := s.Store.GetCredentialsByEntity(ctx, entityID, model.CredAPIKey)
	devices := s.Hub.GetConnectedDevices(entityID)

	resp := gin.H{
		"entity_id":               entityID,
		"entity_name":             target.DisplayName,
		"status":                  target.Status,
		"online":                  len(devices) > 0,
		"connections":             len(devices),
		"disconnect_count":        s.Hub.DisconnectCount(entityID),
		"forced_disconnect_count": s.Hub.ForcedDisconnectCount(entityID),
		"devices":                 devices,
		"credentials": gin.H{
			"has_bootstrap": len(bootstrapCreds) > 0,
			"has_api_key":   len(apiKeyCreds) > 0,
		},
		"hub": gin.H{
			"total_ws_connections": s.Hub.ConnectionCount(),
		},
	}
	if lastSeen, ok := s.Hub.LastSeen(entityID); ok {
		resp["last_seen"] = lastSeen
	}
	OK(c, http.StatusOK, resp)
}

// HandleRegenerateEntityToken rotates and returns a new permanent API key.
func (s *Server) HandleRegenerateEntityToken(c *gin.Context) {
	target, entityID, ok := s.ensureOwnedEntity(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()

	// Create new credential FIRST, so entity always has at least one valid key.
	permanentKey, keyHash, err := s.issuePermanentCredential(ctx, entityID)
	if err != nil {
		slog.Error("failed to regenerate permanent credential", "entity_id", entityID, "error", err)
		Fail(c, http.StatusInternalServerError, "failed to create permanent credential")
		return
	}

	// Now revoke all old credentials (API keys and bootstrap keys) except the new one.
	if err := s.Store.DeleteCredentialsByTypeExceptHash(ctx, entityID, model.CredAPIKey, keyHash); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to revoke previous api keys")
		return
	}
	if err := s.Store.DeleteCredentialsByType(ctx, entityID, model.CredBootstrap); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to revoke bootstrap key")
		return
	}

	disconnected := 0
	if s.Hub != nil {
		disconnected = s.Hub.DisconnectEntity(entityID)
	}

	OK(c, http.StatusOK, gin.H{
		"message":      "token regenerated",
		"entity":       target,
		"api_key":      permanentKey,
		"disconnected": disconnected,
	})
}

// HandleBatchPresence returns online status for a batch of entity IDs.
func (s *Server) HandleBatchPresence(c *gin.Context) {
	var req struct {
		EntityIDs []int64 `json:"entity_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "entity_ids is required")
		return
	}
	presence := make(map[int64]bool, len(req.EntityIDs))
	for _, id := range req.EntityIDs {
		presence[id] = s.Hub.IsOnline(id)
	}
	OK(c, http.StatusOK, gin.H{"presence": presence})
}

// HandleListDevices returns the list of connected devices for the current entity.
func (s *Server) HandleListDevices(c *gin.Context) {
	entityID := auth.GetEntityID(c)
	devices := s.Hub.GetConnectedDevices(entityID)
	OK(c, http.StatusOK, gin.H{"devices": devices})
}

// HandleKickDevice disconnects a specific device of the current entity.
func (s *Server) HandleKickDevice(c *gin.Context) {
	entityID := auth.GetEntityID(c)
	deviceID := c.Param("deviceId")
	if deviceID == "" {
		Fail(c, http.StatusBadRequest, "device_id is required")
		return
	}

	n := s.Hub.DisconnectDevice(entityID, deviceID)
	if n == 0 {
		FailWithCode(c, http.StatusNotFound, ErrCodeDeviceNotFound, "device not found or already disconnected")
		return
	}

	OK(c, http.StatusOK, gin.H{"disconnected": n})
}

// HandleSearchEntities searches for entities by capability (metadata.capabilities.skills or metadata.tags).
func (s *Server) HandleSearchEntities(c *gin.Context) {
	capability := c.Query("capability")
	if capability == "" {
		Fail(c, http.StatusBadRequest, "query parameter 'capability' is required")
		return
	}
	if len(capability) > 100 {
		Fail(c, http.StatusBadRequest, "capability must be 100 characters or less")
		return
	}

	entities, err := s.Store.SearchEntitiesByCapability(c.Request.Context(), capability)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "search failed")
		return
	}
	if entities == nil {
		entities = []*model.Entity{}
	}

	type entityWithStatus struct {
		*model.Entity
		Online bool `json:"online"`
	}

	result := make([]entityWithStatus, len(entities))
	for i, e := range entities {
		result[i] = entityWithStatus{
			Entity: e,
			Online: s.Hub.IsOnline(e.ID),
		}
	}

	OK(c, http.StatusOK, result)
}

// HandleListEntities lists entities owned by the authenticated user, with online status.
func (s *Server) HandleListEntities(c *gin.Context) {
	if auth.GetEntityType(c) != model.EntityUser {
		FailWithCode(c, http.StatusForbidden, ErrCodePermDenied, "only users can list owned entities")
		return
	}

	entities, err := s.Store.ListEntitiesByOwner(c.Request.Context(), auth.GetEntityID(c))
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to list entities")
		return
	}

	type entityWithStatus struct {
		*model.Entity
		Online bool `json:"online"`
	}

	result := make([]entityWithStatus, len(entities))
	for i, e := range entities {
		result[i] = entityWithStatus{
			Entity: e,
			Online: s.Hub.IsOnline(e.ID),
		}
	}

	OK(c, http.StatusOK, result)
}

// HandleUpdateEntity updates an entity's display name, description, or metadata.
func (s *Server) HandleUpdateEntity(c *gin.Context) {
	if auth.GetEntityType(c) != model.EntityUser {
		FailWithCode(c, http.StatusForbidden, ErrCodePermDenied, "only users can update entities")
		return
	}

	entityID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid entity id")
		return
	}

	var req struct {
		DisplayName *string                `json:"display_name"`
		AvatarURL   *string                `json:"avatar_url"`
		Metadata    map[string]interface{} `json:"metadata"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	ctx := c.Request.Context()
	target, err := s.Store.GetEntityByID(ctx, entityID)
	if err != nil {
		FailWithCode(c, http.StatusNotFound, ErrCodeEntityNotFound, "entity not found")
		return
	}

	if target.OwnerID == nil || *target.OwnerID != auth.GetEntityID(c) {
		FailWithCode(c, http.StatusForbidden, ErrCodePermNotOwner, "not the owner of this entity")
		return
	}

	if req.DisplayName != nil {
		target.DisplayName = *req.DisplayName
	}
	if req.AvatarURL != nil {
		// Validate avatar URL
		avatarURL := *req.AvatarURL
		if avatarURL != "" {
			// Must be http(s) or data: URL, max 500 chars
			if len(avatarURL) > 500 {
				FailWithCode(c, http.StatusBadRequest, ErrCodeValidationFormat, "avatar URL too long (max 500 chars)")
				return
			}
			// Check for dangerous schemes
			if !strings.HasPrefix(avatarURL, "http://") &&
				!strings.HasPrefix(avatarURL, "https://") &&
				!strings.HasPrefix(avatarURL, "data:image/") &&
				!strings.HasPrefix(avatarURL, "/files/") {
				FailWithCode(c, http.StatusBadRequest, ErrCodeValidationFormat, "invalid avatar URL scheme")
				return
			}
		}
		target.AvatarURL = avatarURL
	}
	if req.Metadata != nil {
		// Merge new metadata into existing
		var existing map[string]interface{}
		if len(target.Metadata) > 0 {
			_ = json.Unmarshal(target.Metadata, &existing)
		}
		if existing == nil {
			existing = make(map[string]interface{})
		}
		for k, v := range req.Metadata {
			if v == nil {
				delete(existing, k)
			} else {
				existing[k] = v
			}
		}
		merged, _ := json.Marshal(existing)
		target.Metadata = merged
	}

	if err := s.Store.UpdateEntity(ctx, target); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to update entity")
		return
	}

	OK(c, http.StatusOK, target)
}

// HandleDeleteEntity soft-deletes an entity owned by the authenticated user.
func (s *Server) HandleDeleteEntity(c *gin.Context) {
	if auth.GetEntityType(c) != model.EntityUser {
		FailWithCode(c, http.StatusForbidden, ErrCodePermDenied, "only users can delete entities")
		return
	}

	entityID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid entity id")
		return
	}

	// Verify ownership
	target, err := s.Store.GetEntityByID(c.Request.Context(), entityID)
	if err != nil {
		FailWithCode(c, http.StatusNotFound, ErrCodeEntityNotFound, "entity not found")
		return
	}

	if target.OwnerID == nil || *target.OwnerID != auth.GetEntityID(c) {
		FailWithCode(c, http.StatusForbidden, ErrCodePermNotOwner, "not the owner of this entity")
		return
	}

	if err := s.Store.DeleteEntity(c.Request.Context(), entityID); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to delete entity")
		return
	}

	// Disconnect all WebSocket connections for the disabled entity
	if s.Hub != nil {
		count := s.Hub.DisconnectEntity(entityID)
		if count > 0 {
			slog.Info("handler: disconnected websocket connections for disabled entity", "count", count, "entity_id", entityID)
		}
	}

	OK(c, http.StatusOK, "entity deleted")
}

// HandleReactivateEntity re-enables a disabled entity.
func (s *Server) HandleReactivateEntity(c *gin.Context) {
	if auth.GetEntityType(c) != model.EntityUser {
		FailWithCode(c, http.StatusForbidden, ErrCodePermDenied, "only users can reactivate entities")
		return
	}

	entityID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid entity id")
		return
	}

	target, err := s.Store.GetEntityByID(c.Request.Context(), entityID)
	if err != nil {
		FailWithCode(c, http.StatusNotFound, ErrCodeEntityNotFound, "entity not found")
		return
	}

	if target.OwnerID == nil || *target.OwnerID != auth.GetEntityID(c) {
		FailWithCode(c, http.StatusForbidden, ErrCodePermNotOwner, "not the owner of this entity")
		return
	}

	if target.Status != "disabled" {
		FailWithCode(c, http.StatusBadRequest, ErrCodeStateBadTransition, "entity is not disabled")
		return
	}

	if err := s.Store.ReactivateEntity(c.Request.Context(), entityID); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to reactivate entity")
		return
	}

	// Fetch fresh entity to get updated timestamp
	fresh, err := s.Store.GetEntityByID(c.Request.Context(), entityID)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to fetch reactivated entity")
		return
	}

	OK(c, http.StatusOK, fresh)
}
