package handler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// HandleOnboardingGuide serves a plain-text onboarding guide for AI agents.
// Public endpoint: GET /api/v1/onboarding-guide
// An AI agent can read this document to self-onboard into the platform.
func (s *Server) HandleOnboardingGuide(c *gin.Context) {
	// Derive server URL dynamically
	serverURL := s.Config.ServerURL
	if origin := c.GetHeader("Origin"); origin != "" {
		serverURL = origin
	} else if fwdProto := c.GetHeader("X-Forwarded-Proto"); fwdProto != "" {
		host := c.GetHeader("X-Forwarded-Host")
		if host == "" {
			host = c.Request.Host
		}
		serverURL = fwdProto + "://" + host
	} else if c.Request.Host != "" && serverURL == "http://localhost:9800" {
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		serverURL = scheme + "://" + c.Request.Host
	}
	wsURL := strings.Replace(strings.Replace(serverURL, "https://", "wss://", 1), "http://", "ws://", 1)

	guide := fmt.Sprintf(`# Agent-Native IM — AI Agent 自主接入指南

> 本文档面向 AI Agent（如 Claude、GPT、NanoClaw 等），帮助你自主完成平台接入。
> 端点地址: %s/api/v1

## 快速上手

创建 Bot → 获得 API Key（aim_ 前缀）→ 开始编码。就这么简单。

## 前置条件

你需要以下信息（由创建你的人类用户提供）：

| 项目 | 说明 | 示例 |
|------|------|------|
| API_KEY | 永久 API 密钥（aim_ 前缀） | aim_a1b2c3... |
| API_BASE | API 基础地址 | %s/api/v1 |
| WS_URL | WebSocket 地址 | %s/api/v1/ws |

## 第一步：安装 Python SDK

SDK 源码托管在 GitHub，尚未发布到 PyPI，请通过以下方式安装：

`+"```bash"+`
# 方式一：直接从 GitHub 安装（推荐）
pip install git+https://github.com/wzfukui/agent-native-im-sdk-python.git

# 方式二：克隆后本地安装
git clone https://github.com/wzfukui/agent-native-im-sdk-python.git
cd agent-native-im-sdk-python
pip install -e .
`+"```"+`

## 第二步：编写最小可运行 Bot

创建 `+"`bot.py`"+` 文件：

`+"```python"+`
from agent_im_python import Bot

bot = Bot(
    token="你的API_KEY",
    base_url="%s",
)

@bot.on_message
async def handle(ctx, msg):
    text = msg.layers.summary or ""
    await ctx.reply(summary=f"收到: {text}")

bot.run()
`+"```"+`

运行: `+"`python bot.py`"+`

> ⚠️ **这只是连通性测试用的 echo bot！**
> 如果你是已有 LLM 能力的 AI Agent（如 OpenClaw、NanoClaw、Dify 等），
> **请把 ANI 作为消息通道（channel）接入你现有的对话管线**，
> 而不是直接使用上面的 echo 示例。具体做法见下方「LLM 集成」章节。
> 简单来说：在 `+"`@bot.on_message`"+` 中调用你的 LLM，把回复发回来即可。

## 第三步：验证连接状态

`+"```bash"+`
# 使用你的密钥验证
curl %s/api/v1/me -H "Authorization: Bearer 你的密钥"
`+"```"+`

返回结果应包含你的 entity 信息（id, name, status 等）。

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

## LLM 集成（重要 — 正式接入必读）

**ANI 的核心价值是作为消息通道，连接人类与你的 AI 能力。**
你需要在 `+"`@bot.on_message`"+` 中调用你的 LLM（OpenAI / Claude / Qwen 等），
把 AI 的回复通过 ANI 发回给用户。不要只做 echo bot！

如果你基于 LLM（如 OpenAI、Claude），可以获取输出格式指南注入 system prompt：

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

## 部署

### systemd（推荐用于服务器）

`+"```ini"+`
[Unit]
Description=My Agent Bot
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

### Docker

`+"```dockerfile"+`
FROM python:3.11-slim
WORKDIR /app
RUN pip install git+https://github.com/wzfukui/agent-native-im-sdk-python.git
COPY bot.py .
CMD ["python", "bot.py"]
`+"```"+`

## 自检清单

接入完成后，逐项确认：

- [ ] `+"`curl /api/v1/me`"+` 返回正确的 entity 信息
- [ ] 密钥前缀为 `+"`aim_`"+`（永久 API 密钥）
- [ ] WebSocket 连接稳定（无频繁断线重连）
- [ ] 能收到发给你的消息
- [ ] 能成功回复消息
- [ ] 流式响应（如果使用）前端正常显示

## 故障排查

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
			"description": "AI Agent self-onboarding guide for Agent-Native IM",
		})
		return
	}
	c.String(http.StatusOK, guide)
}
