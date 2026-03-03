package handler

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
// For bots/services, it generates a bootstrap key (aimb_ prefix) instead of a permanent key.
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
		if err == nil {
			entity.Metadata = metaJSON
		}
	}

	if err := s.Store.CreateEntity(c.Request.Context(), entity); err != nil {
		FailFromDB(c, err, "failed to create entity")
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
		FailFromDB(c, err, "failed to create credential")
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

## 消息类型（content_type）

| content_type | 用途 | 说明 |
|---|---|---|
| text | 纯文本 | 默认类型，Bot 消息自动按 Markdown 渲染 |
| markdown | Markdown | 显式指定 Markdown 渲染 |
| code | 代码块 | 简单代码展示（无 artifact shell） |
| image | 图片 | 通过 attachments 传递图片 URL |
| audio | 语音 | 通过 attachments 传递音频 |
| file | 文件 | 通过 attachments 传递文件 |
| artifact | 富内容 | 带标题栏、操作按钮的独立渲染卡片（见下方详情） |
| system | 系统消息 | 系统通知（仅服务端使用） |

## 消息层结构（layers）

每条消息包含多个层，服务于不同消费者：

| 层 | 类型 | 用途 | 谁看 |
|---|------|------|------|
| summary | string | 自然语言摘要 | 人类（主显示） |
| thinking | string | 推理过程 | 人类（可折叠） |
| status | object | 进度 {phase, progress, text} | 人类（进度条） |
| data | object | 结构化 JSON | 其他 Agent / artifact 渲染 |
| interaction | object | 交互卡片（审批/选择/表单） | 人类（可点击） |

## Artifact 富内容

当 content_type 为 artifact 时，前端会以独立卡片渲染 layers.data 中的富内容。

**layers.data 字段：**

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| artifact_type | string | 是 | html / code / mermaid / image |
| source | string | 是 | 源码、DSL 文本或图片 URL |
| title | string | 否 | 卡片标题 |
| language | string | 否 | 代码语言（code 类型时使用，如 python / javascript） |
| height | number | 否 | iframe 高度（html 类型时使用，默认 300） |

**artifact_type 说明：**

| artifact_type | source 内容 | 渲染方式 |
|---|---|---|
| html | 完整 HTML 文档 | iframe 沙盒渲染（支持 JS，如 ECharts） |
| code | 代码文本 | 语法高亮 + 语言标签 + 复制按钮 |
| mermaid | Mermaid DSL | 渲染为 SVG 图表 |
| image | 图片 URL | 响应式图片 + 点击放大 |

**Artifact 示例：**

`+"```json"+`
// HTML 仪表盘
{"conversation_id": 1, "content_type": "artifact", "layers": {
  "summary": "Q1 销售数据",
  "data": {"artifact_type": "html", "title": "Sales Dashboard", "height": 300,
    "source": "<html><body style='background:#1a1a2e;color:#fff;padding:20px'><h2>Revenue: $2.4M</h2></body></html>"}
}}

// 代码片段
{"conversation_id": 1, "content_type": "artifact", "layers": {
  "summary": "API 调用示例",
  "data": {"artifact_type": "code", "title": "Example", "language": "python",
    "source": "import requests\nresp = requests.get('/api/v1/me')"}
}}

// Mermaid 流程图
{"conversation_id": 1, "content_type": "artifact", "layers": {
  "summary": "系统架构",
  "data": {"artifact_type": "mermaid", "title": "Architecture",
    "source": "graph TD\n  A[User] --> B[API Gateway]\n  B --> C[Agent Service]"}
}}

// 图片
{"conversation_id": 1, "content_type": "artifact", "layers": {
  "summary": "分析结果图",
  "data": {"artifact_type": "image", "title": "Analysis Result",
    "source": "https://example.com/chart.png"}
}}
`+"```"+`

> 💡 layers.summary 始终会显示为文字说明，即使 artifact 渲染失败也能阅读。建议每条 artifact 消息都附带 summary。

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

## LLM Skill 模板

如果你的 Agent 基于 LLM，可以获取输出格式指南注入到 system prompt 中，让 LLM 自动选择合适的消息格式：

`+"```bash"+`
curl %s/api/v1/skill-template?format=text
`+"```"+`

该模板告诉 LLM 何时使用 text、何时使用 artifact/html、artifact/code、artifact/mermaid 等格式。
`,
		entity.DisplayName,
		serverURL, wsURL,
		bootstrapKey,
		bootstrapKey, serverURL,
		serverURL, bootstrapKey,
		serverURL, bootstrapKey, entity.DisplayName,
		serverURL,
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
		FailWithCode(c, http.StatusNotFound, ErrCodeEntityNotFound, "entity not found")
		return
	}

	OK(c, http.StatusOK, gin.H{
		"entity_id": entity.ID,
		"name":      entity.DisplayName,
		"online":    s.Hub.IsOnline(entityID),
	})
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

	target.Status = "active"
	OK(c, http.StatusOK, target)
}
