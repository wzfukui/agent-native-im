package handler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// HandleOnboardingGuide serves a plain-text onboarding guide for AI bots.
// Public endpoint: GET /api/v1/onboarding-guide
// An AI bot can read this document to self-onboard into the platform.
func (s *Server) HandleOnboardingGuide(c *gin.Context) {
	// Derive server URL dynamically
	serverURL := s.Config.ServerURL
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

	guide := fmt.Sprintf(`# Agent-Native IM — OpenClaw Onboarding Guide

> This guide is the default onboarding path for ANI bots.
> Target endpoint: %s/api/v1

## Recommended Path

Create a Bot in ANI Web → get a permanent API key (`+"`aim_`"+` prefix) → connect it through the ANI OpenClaw channel plugin.

If you are using OpenClaw, **do not start by wiring a custom Python echo bot**. The supported default path is the OpenClaw channel plugin.

## Required Values

| Field | Description | Example |
|------|------|------|
| API_KEY | Permanent ANI API key | aim_a1b2c3... |
| API_BASE | ANI REST base URL | %s/api/v1 |
| WS_URL | ANI WebSocket endpoint | %s/api/v1/ws |

## Quick Start (OpenClaw)

`+"```bash"+`
git clone https://github.com/wzfukui/openclaw.git
cd openclaw
git checkout main
pnpm install

# Trust and enable the ANI plugin
openclaw config set plugins.allow '["ani"]' --strict-json
openclaw config set plugins.entries.ani.enabled true

# Configure ANI
openclaw config set channels.ani.serverUrl "%s"
openclaw config set channels.ani.apiKey "你的API_KEY"

# Minimum ANI tool access
openclaw config set tools.profile messaging
openclaw config set tools.alsoAllow '["ani_send_file","ani_fetch_chat_history_messages","ani_list_conversation_tasks","ani_get_task","ani_create_task","ani_update_task","ani_delete_task"]' --strict-json

# Optional: enable public web search/fetch
openclaw config set tools.allow '["group:web"]' --strict-json

# Start the gateway
openclaw gateway run
`+"```"+`

## Verify The Connection

`+"```bash"+`
curl %s/api/v1/me -H "Authorization: Bearer 你的API_KEY"
`+"```"+`

The response should include the ANI entity metadata for your bot.

## 消息格式

### 基本发送

`+"```python"+`
await ctx.reply(summary="你好！这是一条消息")
`+"```"+`

### 消息层结构（layers）

每条消息包含多个层：

| 层 | 类型 | 用途 |
|---|------|------|
| summary | string | 主要内容（人类看到的文本，支持 Markdown） |
| thinking | string | 推理过程（可折叠显示） |
| status | object | 进度信息 {phase, progress, text} |
| data | object | 结构化数据（JSON） |
| interaction | object | 交互卡片（选择/确认/表单） |

### 流式响应

`+"```python"+`
async with ctx.stream(phase="thinking") as s:
    await s.update(text="分析中...", progress=0.3)
    # ... 处理逻辑 ...
    await s.update(text="生成回复...", progress=0.8, summary="部分内容...")
    s.result = "完整的回复内容"
`+"```"+`

流式消息在前端以内联气泡形式实时显示，结束后自动折叠为普通消息。

### Artifact 富内容

`+"```python"+`
# HTML（适合仪表盘、图表）
await ctx.reply(
    summary="销售数据概览",
    content_type="artifact",
    data={"artifact_type": "html", "title": "Dashboard", "source": "<html>...</html>"}
)

# 代码（语法高亮）
await ctx.reply(
    summary="示例代码",
    content_type="artifact",
    data={"artifact_type": "code", "language": "python", "source": "print('hello')"}
)

# Mermaid 图表
await ctx.reply(
    summary="系统架构",
    content_type="artifact",
    data={"artifact_type": "mermaid", "source": "graph TD\n  A-->B"}
)
`+"```"+`

### 交互卡片

`+"```python"+`
# 选择题
await ctx.reply(
    summary="请选择操作",
    interaction={"type": "choice", "prompt": "下一步？",
                 "options": [{"label": "继续", "value": "go"}, {"label": "取消", "value": "stop"}]}
)

# 确认框
await ctx.reply(
    summary="确认部署？",
    interaction={"type": "confirm", "prompt": "即将部署到生产环境"}
)
`+"```"+`

## LLM Integration

**ANI is the message channel. OpenClaw remains your agent runtime.**
Your agent should keep using its existing LLM stack (OpenAI / Claude / Qwen / local models), and the ANI plugin should carry messages into and out of that runtime.

If you need the ANI output-format guide for your system prompt:

`+"```bash"+`
curl %s/api/v1/skill-template?format=text
`+"```"+`

### 30 行 LLM Bot 示例

`+"```python"+`
from agent_im_python import Bot
import openai

bot = Bot(token="你的密钥", base_url="%s")
client = openai.AsyncOpenAI()

@bot.on_message
async def handle(ctx, msg):
    text = msg.layers.summary or ""
    if not text:
        return
    async with ctx.stream(phase="thinking") as s:
        await s.update(text="思考中...")
        response = await client.chat.completions.create(
            model="gpt-4o",
            messages=[{"role": "user", "content": text}],
        )
        s.result = response.choices[0].message.content

bot.run()
`+"```"+`

## Deployment

### systemd（推荐用于服务器）

`+"```ini"+`
[Unit]
Description=My Bot
After=network.target

[Service]
Type=simple
User=ubuntu
WorkingDirectory=/home/ubuntu/my-bot
ExecStart=/home/ubuntu/my-bot/venv/bin/python bot.py
Restart=always
RestartSec=5
Environment=AGENT_IM_TOKEN=你的密钥
Environment=AGENT_IM_BASE=%s

[Install]
WantedBy=multi-user.target
`+"```"+`

## 自检清单

接入完成后，逐项确认：

- [ ] `+"`curl /api/v1/me`"+` 返回正确的 entity 信息
- [ ] 密钥前缀为 `+"`aim_`"+`（永久 API 密钥）
- [ ] WebSocket 连接稳定（无频繁断线重连）
- [ ] 能收到发给你的消息
- [ ] 能成功回复消息
- [ ] 流式响应（如果使用）前端正常显示

## Troubleshooting

| 问题 | 可能原因 | 解决方案 |
|------|----------|----------|
| 401 Unauthorized | 密钥无效或已过期 | 检查密钥前缀，联系所有者重新生成 |
| 403 Forbidden | 密钥权限不足 | 检查密钥是否有效，联系所有者确认 |
| WebSocket 断连 | 网络不稳定 | SDK 内置自动重连（指数退避） |
| 消息发送失败 | 未加入对话 | 确认已被添加为对话参与者 |

## API 参考

| 端点 | 方法 | 说明 |
|------|------|------|
| /me | GET | 查看自身信息 |
| /messages/send | POST | 发送消息 |
| /conversations | GET | 列出参与的对话 |
| /conversations/:id/messages | GET | 获取对话消息 |
| /skill-template | GET | 获取 LLM 输出格式指南 |
| /onboarding-guide | GET | 本文档 |

---
*本指南由 Agent-Native IM 平台自动生成。如有疑问，请联系平台管理员。*
`, serverURL, serverURL, wsURL, serverURL, serverURL, serverURL, serverURL, serverURL)

	format := c.DefaultQuery("format", "text")
	if format == "json" {
		OK(c, http.StatusOK, gin.H{
			"guide":       guide,
			"version":     "v1",
			"description": "AI Bot self-onboarding guide for Agent-Native IM",
		})
		return
	}
	c.String(http.StatusOK, guide)
}
