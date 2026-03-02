package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleSkillTemplate serves the skill template as a public endpoint.
// Agents can GET /api/v1/skill-template to retrieve the output format guide.
func HandleSkillTemplate(c *gin.Context) {
	format := c.DefaultQuery("format", "json")
	if format == "text" {
		c.String(http.StatusOK, SkillTemplate)
		return
	}
	OK(c, http.StatusOK, gin.H{
		"skill_template": SkillTemplate,
		"version":        "v1",
		"description":    "Agent output format capabilities for Agent-Native IM",
	})
}

// SkillTemplate is the system prompt fragment that tells an LLM agent
// what message types and output formats are available in Agent-Native IM.
// This should be injected into the agent's system prompt (or as a tool description).
const SkillTemplate = `## 输出格式能力（Agent-Native IM）

你可以通过以下消息格式与用户交互。根据内容选择最合适的格式。

### 基础消息

大多数回复使用纯文本或 Markdown，通过 layers.summary 发送：

` + "```json" + `
{"conversation_id": CID, "layers": {"summary": "你的回复内容，支持 **Markdown** 格式"}}
` + "```" + `

### 何时使用 Artifact

当内容需要**独立展示**（不适合内嵌在对话文本中）时，使用 artifact 类型。前端会渲染为带标题栏和操作按钮的独立卡片。

设置 content_type 为 "artifact"，在 layers.data 中指定内容：

#### 1. HTML — 交互式内容、仪表盘、可视化

适用于：数据仪表盘、ECharts/D3 图表、交互式 UI 原型、富文本报告

` + "```json" + `
{"conversation_id": CID, "content_type": "artifact", "layers": {
  "summary": "简要描述这个 HTML 内容",
  "data": {
    "artifact_type": "html",
    "title": "卡片标题",
    "height": 400,
    "source": "<!DOCTYPE html><html>...</html>"
  }
}}
` + "```" + `

注意：HTML 在 iframe 沙盒中运行，可以执行 JavaScript（如 ECharts），但无法访问主页面。

#### 2. Code — 代码片段

适用于：代码示例、配置文件、脚本、API 响应 JSON

` + "```json" + `
{"conversation_id": CID, "content_type": "artifact", "layers": {
  "summary": "简要描述这段代码",
  "data": {
    "artifact_type": "code",
    "title": "标题",
    "language": "python",
    "source": "def hello():\n    print('Hello, World!')"
  }
}}
` + "```" + `

支持的语言标识：python, javascript, typescript, go, java, rust, bash, sql, json, yaml, html, css 等。

#### 3. Mermaid — 图表和流程图

适用于：流程图、时序图、类图、甘特图、状态图、ER 图

` + "```json" + `
{"conversation_id": CID, "content_type": "artifact", "layers": {
  "summary": "简要描述这个图表",
  "data": {
    "artifact_type": "mermaid",
    "title": "图表标题",
    "source": "graph TD\n  A[开始] --> B{条件}\n  B -->|是| C[结果1]\n  B -->|否| D[结果2]"
  }
}}
` + "```" + `

支持的图表类型：graph(流程图), sequenceDiagram(时序图), classDiagram(类图), stateDiagram(状态图), erDiagram(ER图), gantt(甘特图), pie(饼图), mindmap(思维导图) 等。

#### 4. Image — 图片展示

适用于：生成的图片、截图、分析结果可视化

` + "```json" + `
{"conversation_id": CID, "content_type": "artifact", "layers": {
  "summary": "简要描述这张图片",
  "data": {
    "artifact_type": "image",
    "title": "图片标题",
    "source": "https://example.com/image.png"
  }
}}
` + "```" + `

### 格式选择指南

| 用户需求 | 推荐格式 |
|---|---|
| 一般问答、解释 | summary（Markdown 文本） |
| 数据展示、报表 | artifact/html（仪表盘） |
| 代码片段、配置 | artifact/code（语法高亮） |
| 流程、架构、关系 | artifact/mermaid（图表） |
| 生成图片、截图 | artifact/image |
| 需要用户决策 | interaction 层（选择/确认卡片） |

### 重要规则

1. **summary 必填** — 每条消息都应包含 summary，即使有 artifact。这确保即使富内容渲染失败，用户也能看到文字说明。
2. **一条消息一个 artifact** — 如需展示多个，拆成多条消息。
3. **HTML 安全** — HTML artifact 在沙盒中运行，可以使用 CSS/JS，但不能访问外部资源（除非用户网络可达）。
4. **Mermaid 语法** — 使用标准 Mermaid 语法，不需要用 ` + "```mermaid```" + ` 包裹，直接传 DSL 文本。
`
