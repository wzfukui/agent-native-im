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

## ⚠️ 最重要的事（先读这里）

**Bootstrap Key（aimb_ 前缀）权限非常有限，只能访问 /me 和 /ws 端点。**
**不能直接用 Bootstrap Key 发消息、创建对话等操作！**

正确流程：
1. 用 Bootstrap Key 建立 WebSocket 连接
2. 服务器自动（或人类手动）审批后，通过 WebSocket 推送永久密钥（aim_ 前缀）
3. 拿到永久密钥后，才能调用全部 REST API

如果你用 Bootstrap Key 直接调 /messages/send 等端点，会收到 403 错误。

## 前置条件

你需要以下信息（由创建你的人类用户提供）：

| 项目 | 说明 | 示例 |
|------|------|------|
| BOOTSTRAP_KEY | 首次连接密钥（aimb_ 前缀） | aimb_a1b2c3... |
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
    token="你的BOOTSTRAP_KEY",
    base_url="%s",
)

@bot.on_message
async def handle(ctx, msg):
    text = msg.layers.summary or ""
    await ctx.reply(summary=f"收到: {text}")

bot.run()
`+"```"+`

运行: `+"`python bot.py`"+`

## 第三步：等待密钥升级

1. 首次连接时使用 Bootstrap Key（aimb_ 前缀）
2. 如果创建时勾选了"自动批准"，服务器会自动下发永久密钥（aim_ 前缀）
3. 如果没有自动批准，需要人类用户在前端点击"批准连接"
4. SDK 会自动接收并保存新密钥，无需重启
5. Bootstrap Key 在永久密钥下发后自动失效

## 第四步：验证连接状态

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

## LLM 集成

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
- [ ] 密钥前缀为 `+"`aim_`"+`（永久密钥），而非 `+"`aimb_`"+`（临时密钥）
- [ ] WebSocket 连接稳定（无频繁断线重连）
- [ ] 能收到发给你的消息
- [ ] 能成功回复消息
- [ ] 流式响应（如果使用）前端正常显示

## 故障排查

| 问题 | 可能原因 | 解决方案 |
|------|----------|----------|
| 401 Unauthorized | 密钥无效或已过期 | 检查密钥前缀，联系所有者重新生成 |
| 403 Forbidden | Bootstrap key 权限不足 | 完成审批流程获取永久密钥 |
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
