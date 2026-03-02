package handler

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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
	Name        string `json:"name" binding:"required"`
	DisplayName string `json:"display_name"`
	EntityType  string `json:"entity_type"` // defaults to "bot"
}

// generateKey creates a random API key with the given prefix.
// Format: prefix + 48 hex chars (24 random bytes).
func generateKey(prefix string) string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

// HandleCreateEntity creates a bot/service entity owned by the authenticated user.
// For bots/services, it generates a bootstrap key (aimb_ prefix) instead of a permanent key.
func (s *Server) HandleCreateEntity(c *gin.Context) {
	if auth.GetEntityType(c) != model.EntityUser {
		Fail(c, http.StatusForbidden, "only users can create entities")
		return
	}

	var req createEntityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "name is required")
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

	if err := s.Store.CreateEntity(c.Request.Context(), entity); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to create entity")
		return
	}

	// Generate bootstrap key
	bootstrapKey := generateKey(keyPrefixBootstrap)
	keyHash := fmt.Sprintf("%x", sha256.Sum256([]byte(bootstrapKey)))

	cred := &model.Credential{
		EntityID:   entity.ID,
		CredType:   model.CredBootstrap,
		SecretHash: keyHash,
		RawPrefix:  bootstrapKey[:8],
	}

	if err := s.Store.CreateCredential(c.Request.Context(), cred); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to create credential")
		return
	}

	// Build markdown onboarding doc (SKILL format)
	serverURL := s.Config.ServerURL
	wsURL := strings.Replace(strings.Replace(serverURL, "https://", "wss://", 1), "http://", "ws://", 1)

	markdownDoc := fmt.Sprintf(`# Agent 接入指南 — %s

## 连接信息

| 项目 | 值 |
|------|---|
| API Base | `+"`%s/api/v1`"+` |
| WebSocket | `+"`%s/api/v1/ws?token=YOUR_KEY`"+` |
| Bootstrap Key | `+"`%s`"+` |

> ⚠️ Bootstrap Key 仅供首次接入使用。Agent 连接并经用户确认后，服务器下发永久密钥（aim_ 前缀），此密钥自动失效。

## 快速接入（Python SDK）

`+"```python"+`
from agent_im_python import Bot

bot = Bot(token="%s", base_url="%s")

@bot.on_message
async def handle(ctx, msg):
    text = msg.layers.summary or ""
    await ctx.reply(summary=f"收到: {text}")

bot.run()
`+"```"+`

## 快速接入（HTTP）

`+"```bash"+`
# 验证连接
curl %s/api/v1/me -H "Authorization: Bearer %s"

# 发送消息
curl -X POST %s/api/v1/messages/send \
  -H "Authorization: Bearer %s" \
  -H "Content-Type: application/json" \
  -d '{"conversation_id": 1, "layers": {"summary": "Hello from %s!"}}'
`+"```"+`

## 消息层结构

每条消息包含多个层，服务于不同消费者：

| 层 | 类型 | 用途 | 谁看 |
|---|------|------|------|
| summary | string | 自然语言摘要 | 人类（主显示） |
| thinking | string | 推理过程 | 人类（可折叠） |
| status | object | 进度 {phase, progress, text} | 人类（进度条） |
| data | object | 结构化 JSON | 其他 Agent |
| interaction | object | 交互卡片（审批/选择/表单） | 人类（可点击） |

## 流式响应协议

通过 WebSocket 发送流式消息，实时展示处理过程：

`+"```"+`
stream_start  → 开启流，显示状态面板（不持久化）
stream_delta  → 更新进度/内容（不持久化，0~N 次）
stream_end    → 最终结果（持久化到数据库）
`+"```"+`

`+"```json"+`
{"type": "message.send", "data": {
  "conversation_id": 1,
  "stream_id": "task-001",
  "stream_type": "start",
  "layers": {"status": {"phase": "thinking", "progress": 0, "text": "分析中..."}}
}}
`+"```"+`
`,
		entity.DisplayName,
		serverURL, wsURL,
		bootstrapKey,
		bootstrapKey, serverURL,
		serverURL, bootstrapKey,
		serverURL, bootstrapKey, entity.DisplayName,
	)

	OK(c, http.StatusCreated, gin.H{
		"entity":       entity,
		"bootstrap_key": bootstrapKey,
		"markdown_doc": markdownDoc,
	})
}

// HandleApproveConnection approves an Agent's connection request.
// It generates a permanent API key, deletes the bootstrap key, and pushes the new key via WebSocket.
func (s *Server) HandleApproveConnection(c *gin.Context) {
	if auth.GetEntityType(c) != model.EntityUser {
		Fail(c, http.StatusForbidden, "only users can approve connections")
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
		Fail(c, http.StatusNotFound, "entity not found")
		return
	}

	if target.OwnerID == nil || *target.OwnerID != auth.GetEntityID(c) {
		Fail(c, http.StatusForbidden, "not the owner of this entity")
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

	// Push new key to the Agent via WebSocket
	s.Hub.SendToEntity(entityID, ws.WSMessage{
		Type: "connection.approved",
		Data: gin.H{
			"api_key": permanentKey,
			"message": "Connection approved. Use this permanent key for all future requests.",
		},
	})

	OK(c, http.StatusOK, gin.H{
		"message": "connection approved",
		"entity":  target,
	})
}

// HandleEntityStatus returns the online status of an entity.
func (s *Server) HandleEntityStatus(c *gin.Context) {
	entityID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid entity id")
		return
	}

	entity, err := s.Store.GetEntityByID(c.Request.Context(), entityID)
	if err != nil {
		Fail(c, http.StatusNotFound, "entity not found")
		return
	}

	OK(c, http.StatusOK, gin.H{
		"entity_id": entity.ID,
		"name":      entity.DisplayName,
		"online":    s.Hub.IsOnline(entityID),
	})
}

// HandleListEntities lists entities owned by the authenticated user, with online status.
func (s *Server) HandleListEntities(c *gin.Context) {
	if auth.GetEntityType(c) != model.EntityUser {
		Fail(c, http.StatusForbidden, "only users can list owned entities")
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

// HandleDeleteEntity soft-deletes an entity owned by the authenticated user.
func (s *Server) HandleDeleteEntity(c *gin.Context) {
	if auth.GetEntityType(c) != model.EntityUser {
		Fail(c, http.StatusForbidden, "only users can delete entities")
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
		Fail(c, http.StatusNotFound, "entity not found")
		return
	}

	if target.OwnerID == nil || *target.OwnerID != auth.GetEntityID(c) {
		Fail(c, http.StatusForbidden, "not the owner of this entity")
		return
	}

	if err := s.Store.DeleteEntity(c.Request.Context(), entityID); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to delete entity")
		return
	}

	OK(c, http.StatusOK, "entity deleted")
}
