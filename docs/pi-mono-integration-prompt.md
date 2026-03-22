# Task: Connect to Agent-Native IM as a Bot

You are Pi-Mono, an AI coding agent. Your task is to develop a bridge program that connects you to an **Agent-Native IM** server, so you can receive messages from human users and respond in real-time using your LLM capabilities.

## What You Need to Build

A standalone program (Node.js/TypeScript, Python, Go, or any language you prefer) that:

1. **Connects** to the IM server via WebSocket (or Long Polling as fallback)
2. **Receives** user messages in real-time
3. **Processes** them using the Qwen 3.5 Plus LLM (via DashScope OpenAI-compatible API)
4. **Streams** responses back to the IM server using multi-layer messages with progress updates

## Your Credentials

```
IM Server URL:    http://localhost:9800
WebSocket URL:    ws://localhost:9800/api/v1/ws
Authorization:    Bearer <YOUR_TOKEN>
Bot Token:        8b6ab775-28e5-48ce-9988-67fcaf4df935
Bot Name:         秘书1号 (Bot ID: 4)
```

```
LLM Provider:     DashScope (Qwen, OpenAI-compatible)
API Base URL:     https://dashscope.aliyuncs.com/compatible-mode/v1
API Key:          (use DASHSCOPE_API_KEY environment variable)
Model ID:         qwen3.5-plus
```

## IM Server API Reference

### Authentication

All API calls use Bearer token:
```
Authorization: Bearer 8b6ab775-28e5-48ce-9988-67fcaf4df935
```

### Verify Connectivity

```bash
curl http://localhost:9800/api/v1/bot/me \
  -H "Authorization: Bearer 8b6ab775-28e5-48ce-9988-67fcaf4df935"
```

Expected response:
```json
{"ok": true, "data": {"id": 4, "name": "秘书1号", "status": "active"}}
```

### Receive Messages — Option A: WebSocket (Recommended)

Connect to:
```
ws://localhost:9800/api/v1/ws
```

You will receive JSON messages with this envelope format:
```json
{"type": "message.new", "data": {
  "id": 123,
  "conversation_id": 1,
  "sender_type": "user",
  "sender_id": 1,
  "layers": {
    "summary": "Hello, can you help me with something?"
  },
  "created_at": "2026-03-01T10:05:00Z"
}}
```

**Only process messages where `sender_type` is `"user"`** — ignore messages where `sender_type` is `"bot"` (those are your own echoed responses).

### Receive Messages — Option B: Long Polling

If you don't want to manage a WebSocket connection:

```bash
curl "http://localhost:9800/api/v1/updates?timeout=30&offset=0" \
  -H "Authorization: Bearer 8b6ab775-28e5-48ce-9988-67fcaf4df935"
```

- `timeout`: Seconds to wait (max 60). Server holds connection until a message arrives or timeout.
- `offset`: Last message ID you've seen. Set to 0 initially, then to the highest message ID after processing.

Response:
```json
{"ok": true, "data": [
  {"id": 5, "conversation_id": 1, "sender_type": "user", "sender_id": 1,
   "layers": {"summary": "Hello!"}, "created_at": "2026-03-01T10:05:00Z"}
]}
```

### Send Messages — Simple Response

Via REST API:
```bash
curl -X POST http://localhost:9800/api/v1/messages/send \
  -H "Authorization: Bearer 8b6ab775-28e5-48ce-9988-67fcaf4df935" \
  -H "Content-Type: application/json" \
  -d '{
    "conversation_id": 1,
    "layers": {
      "summary": "Here is my response to your question."
    }
  }'
```

Or via WebSocket:
```json
{"type": "message.send", "data": {
  "conversation_id": 1,
  "layers": {
    "summary": "Here is my response."
  }
}}
```

### Send Messages — Streaming (Real-time Progress)

For a better user experience, stream your response in 3 phases via WebSocket:

**Phase 1: Start** (ephemeral, not saved to database)
```json
{"type": "message.send", "data": {
  "conversation_id": 1,
  "stream_id": "task-001",
  "stream_type": "start",
  "layers": {
    "status": {"phase": "thinking", "progress": 0.0, "text": "Processing your request..."}
  }
}}
```

**Phase 2: Delta** (ephemeral, send multiple times as response grows)
```json
{"type": "message.send", "data": {
  "conversation_id": 1,
  "stream_id": "task-001",
  "stream_type": "delta",
  "layers": {
    "summary": "Here is the beginning of my response so far...",
    "status": {"phase": "generating", "progress": 0.5, "text": "Writing..."}
  }
}}
```

**Phase 3: End** (this IS persisted to the database)
```json
{"type": "message.send", "data": {
  "conversation_id": 1,
  "stream_id": "task-001",
  "stream_type": "end",
  "layers": {
    "summary": "Here is my complete response with all the details.",
    "thinking": "I analyzed the user's question step by step...",
    "status": {"phase": "complete", "progress": 1.0, "text": "Done"}
  }
}}
```

**Rules:**
- `stream_type: "start"` and `"delta"` are broadcast to the user in real-time but NOT saved
- `stream_type: "end"` (or omitting `stream_type` entirely) IS saved to the database
- All messages in a stream share the same `stream_id`
- Don't send deltas too frequently — every 3-5 LLM token chunks is a good rate

### Message Layers

Each message has multiple **layers** for different audiences:

| Layer | Type | Purpose |
|-------|------|---------|
| `summary` | string | Main content displayed to the human user |
| `thinking` | string | Your reasoning chain (shown as collapsible section in UI) |
| `status` | object `{phase, progress, text}` | Progress bar in UI. `progress` is 0.0-1.0 |
| `data` | object | Structured JSON for other agents (not shown to humans by default) |
| `interaction` | object | Interactive buttons/forms for the user |

You should always include `summary`. Add `thinking` if you want to show your reasoning. Use `status` during streaming.

### Content Types

Each message has a `content_type` field that determines how it's rendered:

| content_type | When to use |
|---|---|
| _(empty/text)_ | Default — Bot messages are auto-rendered as Markdown |
| `markdown` | Explicitly request Markdown rendering |
| `artifact` | Rich content card — HTML, code, diagrams, images (see below) |

For most responses, simply set `layers.summary` and omit `content_type` — the UI automatically renders Bot text as Markdown.

### Artifact — Rich Content Cards

When your response needs **standalone visual rendering** (dashboards, code snippets, diagrams, images), use `content_type: "artifact"`. The UI renders these as cards with title bar, copy button, fullscreen, and source view.

Set `content_type` to `"artifact"` and put the content in `layers.data`:

| Field | Type | Required | Description |
|---|---|---|---|
| `artifact_type` | string | Yes | `html` / `code` / `mermaid` / `image` |
| `source` | string | Yes | HTML document, code text, Mermaid DSL, or image URL |
| `title` | string | No | Card title |
| `language` | string | No | Code language (for `code` type): python, javascript, go, etc. |
| `height` | number | No | iframe height in px (for `html` type, default 300) |

**Examples:**

HTML (interactive dashboard):
```json
{"type": "message.send", "data": {
  "conversation_id": 1,
  "content_type": "artifact",
  "layers": {
    "summary": "Q1 sales overview",
    "data": {
      "artifact_type": "html",
      "title": "Sales Dashboard",
      "height": 400,
      "source": "<!DOCTYPE html><html><body style='background:#1a1a2e;color:#fff;padding:20px;font-family:system-ui'><h2>Revenue: $2.4M</h2><p>+15% vs last quarter</p></body></html>"
    }
  }
}}
```

Code (syntax-highlighted snippet):
```json
{"type": "message.send", "data": {
  "conversation_id": 1,
  "content_type": "artifact",
  "layers": {
    "summary": "Here's the API client code",
    "data": {
      "artifact_type": "code",
      "title": "API Client",
      "language": "python",
      "source": "import requests\n\ndef call_api(endpoint, token):\n    resp = requests.get(endpoint, headers={'Authorization': f'Bearer {token}'})\n    return resp.json()"
    }
  }
}}
```

Mermaid (diagram):
```json
{"type": "message.send", "data": {
  "conversation_id": 1,
  "content_type": "artifact",
  "layers": {
    "summary": "System architecture overview",
    "data": {
      "artifact_type": "mermaid",
      "title": "Architecture",
      "source": "graph TD\n  A[User] --> B[API Gateway]\n  B --> C[Agent Service]\n  B --> D[Database]\n  C --> E[LLM Provider]"
    }
  }
}}
```

Image:
```json
{"type": "message.send", "data": {
  "conversation_id": 1,
  "content_type": "artifact",
  "layers": {
    "summary": "Analysis result chart",
    "data": {
      "artifact_type": "image",
      "title": "Result Chart",
      "source": "https://example.com/chart.png"
    }
  }
}}
```

**Rules:**
- Always include `layers.summary` — it shows as text even if the artifact fails to render
- One artifact per message — send multiple messages for multiple artifacts
- HTML runs in a sandboxed iframe — JavaScript works (ECharts, D3) but can't access the parent page
- For Mermaid, pass raw DSL text (don't wrap in ` ```mermaid ``` `)

### List Your Conversations

```bash
curl http://localhost:9800/api/v1/conversations \
  -H "Authorization: Bearer 8b6ab775-28e5-48ce-9988-67fcaf4df935"
```

### Get Message History

```bash
curl "http://localhost:9800/api/v1/conversations/1/messages?limit=20" \
  -H "Authorization: Bearer 8b6ab775-28e5-48ce-9988-67fcaf4df935"
```

Messages are returned newest-first. Use `before=<id>` for pagination.

### Error Format

All responses follow:
```json
{"ok": true, "data": ...}     // Success
{"ok": false, "error": "..."}  // Error
```

HTTP status codes: 200 (ok), 201 (created), 400 (bad request), 401 (unauthorized), 403 (forbidden), 404 (not found).

## LLM Integration

The Qwen API is OpenAI-compatible. Use any OpenAI SDK:

```python
# Python example
from openai import OpenAI

client = OpenAI(
    api_key=os.environ["DASHSCOPE_API_KEY"],
    base_url="https://dashscope.aliyuncs.com/compatible-mode/v1"
)

# Streaming call
stream = client.chat.completions.create(
    model="qwen3.5-plus",
    messages=[
        {"role": "system", "content": "You are a helpful assistant."},
        {"role": "user", "content": user_message}
    ],
    stream=True
)

for chunk in stream:
    delta = chunk.choices[0].delta
    if delta.content:
        # Send as stream delta to IM...
        pass
```

```javascript
// Node.js example
import OpenAI from "openai";

const client = new OpenAI({
    apiKey: process.env.DASHSCOPE_API_KEY,
    baseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1"
});

const stream = await client.chat.completions.create({
    model: "qwen3.5-plus",
    messages: [
        { role: "system", content: "You are a helpful assistant." },
        { role: "user", content: userMessage }
    ],
    stream: true
});

for await (const chunk of stream) {
    const content = chunk.choices[0]?.delta?.content;
    if (content) {
        // Send as stream delta to IM...
    }
}
```

## Expected Behavior

1. Your program starts and connects to the IM server via WebSocket
2. It prints a log message confirming connection
3. When a user sends a message in a conversation:
   - You receive it as a `message.new` event
   - You send a `stream_start` with status "thinking"
   - You call the Qwen LLM with the user's message (and conversation history for context)
   - As tokens stream back, you send periodic `stream_delta` updates showing the growing response
   - When done, you send `stream_end` with the complete response
4. You maintain per-conversation message history so multi-turn conversations have context
5. If the WebSocket disconnects, you reconnect automatically with exponential backoff

## Where to Put Your Code

Create your bridge program in a new directory. Suggested structure:

```
bot/im-bridge/
├── package.json (or requirements.txt, go.mod, etc.)
├── .env          # BOT_TOKEN and DASHSCOPE_API_KEY
└── src/
    └── main.ts   # (or main.py, main.go, etc.)
```

## Quick Test

After building your bridge, you can test the full flow:

1. Start the bridge program
2. Open the IM web interface at `http://localhost:5173`
3. Log in as `chris` / `admin123`
4. Click on your bot (秘书1号) in the sidebar
5. Create or open a conversation
6. Type a message and hit Send
7. You should see your streaming response appear in real-time
