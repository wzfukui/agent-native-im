// Mirrors Go models in internal/model/

export interface User {
  id: number
  username: string
  created_at: string
}

export interface Bot {
  id: number
  owner_id: number
  name: string
  status: string
  token?: string // only returned on creation
  created_at: string
}

export interface Conversation {
  id: number
  user_id: number
  bot_id: number
  title: string
  created_at: string
  updated_at: string
  bot?: Bot
}

export interface StatusLayer {
  phase: string
  progress: number
  text: string
}

export interface InteractionOption {
  label: string
  value: string
}

export interface Interaction {
  type: 'approval' | 'choice' | 'form'
  prompt: string
  options: InteractionOption[]
}

export interface MessageLayers {
  thinking?: string
  status?: StatusLayer
  data?: Record<string, unknown>
  summary?: string
  interaction?: Interaction
}

export interface Message {
  id: number
  conversation_id: number
  stream_id?: string
  sender_type: 'user' | 'bot'
  sender_id: number
  layers: MessageLayers
  created_at: string
}

// WebSocket message types

export interface WSOutbound {
  type: 'message.new' | 'stream.start' | 'stream.delta' | 'stream.end' | 'pong' | 'error'
  data: Message | { message?: string }
}

export interface WSInbound {
  type: 'message.send' | 'ping'
  data?: {
    conversation_id: number
    layers?: MessageLayers
    stream_id?: string
    stream_type?: 'start' | 'delta' | 'end'
  }
}

export interface LoginResponse {
  token: string
  user: User
}
