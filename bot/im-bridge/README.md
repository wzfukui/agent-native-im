# IM Bridge - Agent-Native IM Integration

Bridge program connecting Pi-Mono AI agent to Agent-Native IM server.

## Features

- ✅ WebSocket connection to IM server
- ✅ Real-time message reception
- ✅ Streaming responses with progress updates
- ✅ Multi-layer message support (summary, thinking, status)
- ✅ Per-conversation history tracking
- ✅ Automatic reconnection with backoff
- ✅ Qwen 3.5 Plus LLM integration via DashScope

## Quick Start

```bash
# Install dependencies
npm install

# Run in development mode
npm run dev

# Build for production
npm run build

# Run production build
npm start
```

## Configuration

Edit `.env` file:

```env
# IM Server Configuration
BOT_TOKEN=your-bot-token
BOT_NAME=Your Bot Name
IM_SERVER_URL=http://localhost:9800
WS_URL=ws://localhost:9800/api/v1/ws

# LLM Configuration
DASHSCOPE_API_KEY=your-dashscope-api-key
LLM_BASE_URL=https://dashscope.aliyuncs.com/compatible-mode/v1
LLM_MODEL=qwen3.5-plus
```

## Testing

1. Start the bridge: `npm run dev`
2. Open IM web interface: `http://localhost:5173`
3. Log in as `chris` / `admin123`
4. Click on 秘书 1号 bot in sidebar
5. Send a message and observe streaming response

## Architecture

```
┌─────────────┐     WebSocket      ┌─────────────┐
│   IM Server │◄──────────────────►│  IM Bridge  │
│  (port 9800)│     message.new    │ (this app)  │
└─────────────┘                    └──────┬──────┘
                                          │
                                          │ OpenAI API
                                          ▼
                                   ┌─────────────┐
                                   │  DashScope  │
                                   │  Qwen 3.5   │
                                   └─────────────┘
```

## Message Flow

1. User sends message → IM Server
2. IM Server → WebSocket → Bridge (`message.new`)
3. Bridge → LLM (with conversation history)
4. LLM streams tokens → Bridge
5. Bridge → WebSocket → IM Server (`stream_start`, `stream_delta`, `stream_end`)
6. IM Server → User (real-time streaming display)

## Project Structure

```
bot/im-bridge/
├── package.json      # Dependencies and scripts
├── tsconfig.json     # TypeScript configuration
├── .env              # Environment variables (gitignored)
├── .env.example      # Example environment file
├── README.md         # This file
└── src/
    └── main.ts       # Main bridge implementation
```
