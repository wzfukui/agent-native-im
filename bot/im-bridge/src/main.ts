import WebSocket from "ws";
import OpenAI from "openai";
import { ChatCompletionChunk } from "openai/resources/chat/completions";
import * as dotenv from "dotenv";
import { readFileSync } from "fs";
import { join, dirname } from "path";
import { fileURLToPath } from "url";

// Load environment variables
const __dirname = dirname(fileURLToPath(import.meta.url));
const envPath = join(__dirname, "..", ".env");

// Parse .env file manually since we're using ES modules
const envContent = readFileSync(envPath, "utf-8");
const envVars: Record<string, string> = {};
envContent.split("\n").forEach((line) => {
  const trimmed = line.trim();
  if (trimmed && !trimmed.startsWith("#")) {
    const [key, ...valueParts] = trimmed.split("=");
    if (key) {
      envVars[key.trim()] = valueParts.join("=").trim();
    }
  }
});

// Configuration
const CONFIG = {
  botToken: envVars.BOT_TOKEN || "",
  botName: envVars.BOT_NAME || "Bot",
  imServerUrl: envVars.IM_SERVER_URL || "http://localhost:9800",
  wsUrl: envVars.WS_URL || "ws://localhost:9800/api/v1/ws",
  dashscopeApiKey: envVars.DASHSCOPE_API_KEY || "",
  llmBaseUrl: envVars.LLM_BASE_URL || "https://dashscope.aliyuncs.com/compatible-mode/v1",
  llmModel: envVars.LLM_MODEL || "qwen3.5-plus",
};

// Message layer types
interface StatusLayer {
  phase: "thinking" | "generating" | "complete" | "error";
  progress: number;
  text: string;
}

interface MessageLayers {
  summary?: string;
  thinking?: string;
  status?: StatusLayer;
  data?: Record<string, unknown>;
  interaction?: Record<string, unknown>;
}

interface IncomingMessage {
  id: number;
  conversation_id: number;
  sender_type: "user" | "bot";
  sender_id: number;
  layers: MessageLayers;
  created_at: string;
}

interface WebSocketMessage {
  type: string;
  data: IncomingMessage | SendMessageData;
}

interface SendMessageData {
  conversation_id: number;
  stream_id?: string;
  stream_type?: "start" | "delta" | "end";
  layers: MessageLayers;
}

// Conversation history storage
interface ConversationContext {
  messages: Array<{ role: "user" | "assistant"; content: string }>;
  lastMessageId: number;
}

const conversationHistory = new Map<number, ConversationContext>();

// Initialize OpenAI client for DashScope
const openai = new OpenAI({
  apiKey: CONFIG.dashscopeApiKey,
  baseURL: CONFIG.llmBaseUrl,
});

// Generate unique stream ID
function generateStreamId(): string {
  return `stream-${Date.now()}-${Math.random().toString(36).substring(2, 9)}`;
}

// Send message via REST API
async function sendRestMessage(data: SendMessageData): Promise<void> {
  const url = `${CONFIG.imServerUrl}/api/v1/messages/send`;
  const response = await fetch(url, {
    method: "POST",
    headers: {
      "Authorization": `Bearer ${CONFIG.botToken}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify(data),
  });

  const result = await response.json();
  if (!result.ok) {
    console.error("Failed to send message:", result.error);
  }
}

// Send WebSocket message
function sendWsMessage(ws: WebSocket, data: SendMessageData): void {
  const message: WebSocketMessage = {
    type: "message.send",
    data,
  };
  ws.send(JSON.stringify(message));
}

// Process LLM response with streaming
async function processWithLLM(
  ws: WebSocket,
  conversationId: number,
  userMessage: string,
  streamId: string
): Promise<void> {
  const history = conversationHistory.get(conversationId);
  const messages = [
    { role: "system" as const, content: "You are a helpful AI assistant integrated with an enterprise IM system. Provide clear, concise, and helpful responses." },
    ...(history?.messages || []),
    { role: "user" as const, content: userMessage },
  ];

  // Send start message
  sendWsMessage(ws, {
    conversation_id: conversationId,
    stream_id: streamId,
    stream_type: "start",
    layers: {
      status: {
        phase: "thinking",
        progress: 0.0,
        text: "Processing your request...",
      },
    },
  });

  try {
    const stream = await openai.chat.completions.create({
      model: CONFIG.llmModel,
      messages,
      stream: true,
    });

    let fullResponse = "";
    let chunkCount = 0;
    const totalExpectedChunks = 100; // Estimate for progress calculation

    // Send initial delta with thinking status
    sendWsMessage(ws, {
      conversation_id: conversationId,
      stream_id: streamId,
      stream_type: "delta",
      layers: {
        summary: "",
        status: {
          phase: "generating",
          progress: 0.1,
          text: "Generating response...",
        },
      },
    });

    for await (const chunk of stream as AsyncIterable<ChatCompletionChunk>) {
      const content = chunk.choices[0]?.delta?.content || "";
      if (content) {
        fullResponse += content;
        chunkCount++;

        // Send delta every 5 chunks to avoid flooding
        if (chunkCount % 5 === 0) {
          const progress = Math.min(0.9, 0.1 + (chunkCount / totalExpectedChunks) * 0.8);
          sendWsMessage(ws, {
            conversation_id: conversationId,
            stream_id: streamId,
            stream_type: "delta",
            layers: {
              summary: fullResponse,
              status: {
                phase: "generating",
                progress,
                text: `Writing... ${Math.round(progress * 100)}%`,
              },
            },
          });
        }
      }
    }

    // Update conversation history
    if (!history) {
      conversationHistory.set(conversationId, {
        messages: [
          { role: "user", content: userMessage },
          { role: "assistant", content: fullResponse },
        ],
        lastMessageId: 0,
      });
    } else {
      history.messages.push(
        { role: "user", content: userMessage },
        { role: "assistant", content: fullResponse }
      );
      // Keep only last 20 messages to avoid context overflow
      if (history.messages.length > 20) {
        history.messages = history.messages.slice(-20);
      }
    }

    // Send end message with complete response
    sendWsMessage(ws, {
      conversation_id: conversationId,
      stream_id: streamId,
      stream_type: "end",
      layers: {
        summary: fullResponse,
        thinking: `Processed user message in conversation ${conversationId}. Response generated using ${CONFIG.llmModel}.`,
        status: {
          phase: "complete",
          progress: 1.0,
          text: "Done",
        },
      },
    });

    console.log(`[Conversation ${conversationId}] Response sent: ${fullResponse.substring(0, 50)}...`);
  } catch (error) {
    console.error("LLM error:", error);
    sendWsMessage(ws, {
      conversation_id: conversationId,
      stream_id: streamId,
      stream_type: "end",
      layers: {
        summary: "I apologize, but I encountered an error processing your request. Please try again.",
        status: {
          phase: "error",
          progress: 1.0,
          text: "Error occurred",
        },
      },
    });
  }
}

// Handle incoming message
function handleIncomingMessage(ws: WebSocket, message: IncomingMessage): void {
  // Only process user messages
  if (message.sender_type !== "user") {
    console.log(`[Ignored] Message from ${message.sender_type} in conversation ${message.conversation_id}`);
    return;
  }

  const userMessage = message.layers.summary || "";
  if (!userMessage) {
    console.log("[Ignored] Empty message");
    return;
  }

  console.log(
    `[Received] Conversation ${message.conversation_id} from user ${message.sender_id}: ${userMessage.substring(0, 50)}...`
  );

  // Update last message ID for this conversation
  const history = conversationHistory.get(message.conversation_id);
  if (history) {
    history.lastMessageId = message.id;
  }

  // Process with LLM
  const streamId = generateStreamId();
  processWithLLM(ws, message.conversation_id, userMessage, streamId);
}

// Create and manage WebSocket connection
function createWebSocket(): WebSocket {
  const wsUrl = `${CONFIG.wsUrl}?token=${CONFIG.botToken}`;
  console.log(`Connecting to IM server: ${wsUrl}`);

  const ws = new WebSocket(wsUrl);

  ws.on("open", () => {
    console.log("✅ Connected to IM server successfully!");
    console.log(`Bot: ${CONFIG.botName} (${CONFIG.botToken.substring(0, 8)}...)`);
  });

  ws.on("message", (data: Buffer) => {
    try {
      const message: WebSocketMessage = JSON.parse(data.toString());
      console.log("[WS Event]", message.type);

      if (message.type === "message.new" && "data" in message) {
        const incomingMessage = message.data as IncomingMessage;
        handleIncomingMessage(ws, incomingMessage);
      }
    } catch (error) {
      console.error("Failed to parse message:", error);
    }
  });

  ws.on("error", (error) => {
    console.error("WebSocket error:", error.message);
  });

  ws.on("close", (code, reason) => {
    console.log(`WebSocket closed: ${code} ${reason}`);
    // Reconnect with exponential backoff
    setTimeout(() => {
      console.log("Attempting to reconnect...");
      createWebSocket();
    }, 3000);
  });

  return ws;
}

// Verify bot connectivity
async function verifyConnectivity(): Promise<boolean> {
  try {
    const response = await fetch(`${CONFIG.imServerUrl}/api/v1/bot/me`, {
      headers: {
        Authorization: `Bearer ${CONFIG.botToken}`,
      },
    });

    const result = await response.json();
    if (result.ok) {
      console.log("✅ Bot verification successful:", result.data);
      return true;
    } else {
      console.error("❌ Bot verification failed:", result.error);
      return false;
    }
  } catch (error) {
    console.error("❌ Failed to verify bot:", error);
    return false;
  }
}

// Main entry point
async function main(): Promise<void> {
  console.log("🚀 Starting IM Bridge...");
  console.log("Configuration:");
  console.log(`  IM Server: ${CONFIG.imServerUrl}`);
  console.log(`  Bot Name: ${CONFIG.botName}`);
  console.log(`  LLM Model: ${CONFIG.llmModel}`);
  console.log("");

  // Verify connectivity first
  const verified = await verifyConnectivity();
  if (!verified) {
    console.error("Cannot proceed without bot verification. Exiting.");
    process.exit(1);
  }

  // Check if DASHSCOPE_API_KEY is set
  if (!CONFIG.dashscopeApiKey) {
    console.warn("⚠️  DASHSCOPE_API_KEY not set. LLM calls will fail.");
    console.warn("   Please set it in .env file or environment variable.");
  }

  console.log("");
  console.log("📡 Connecting to WebSocket...");

  // Create WebSocket connection
  createWebSocket();

  // Handle graceful shutdown
  process.on("SIGINT", () => {
    console.log("\n👋 Shutting down...");
    process.exit(0);
  });
}

// Run
main().catch((error) => {
  console.error("Fatal error:", error);
  process.exit(1);
});
