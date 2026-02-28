import type { Bot, Conversation, LoginResponse, Message, MessageLayers } from './types'

const BASE = '/api/v1'

let _token: string | null = localStorage.getItem('token')

export function setToken(token: string | null) {
  _token = token
  if (token) {
    localStorage.setItem('token', token)
  } else {
    localStorage.removeItem('token')
  }
}

export function getToken(): string | null {
  return _token
}

async function api<T>(path: string, options?: RequestInit): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options?.headers as Record<string, string>),
  }
  if (_token) {
    headers['Authorization'] = `Bearer ${_token}`
  }

  const res = await fetch(`${BASE}${path}`, { ...options, headers })
  const json = await res.json()

  if (!json.ok) {
    throw new Error(json.error || `API error: ${res.status}`)
  }
  return json.data as T
}

export async function login(username: string, password: string): Promise<LoginResponse> {
  return api<LoginResponse>('/auth/login', {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  })
}

export async function listBots(): Promise<Bot[]> {
  return api<Bot[]>('/bots')
}

export async function createBot(name: string): Promise<Bot> {
  return api<Bot>('/bots', {
    method: 'POST',
    body: JSON.stringify({ name }),
  })
}

export async function deleteBot(id: number): Promise<void> {
  await api(`/bots/${id}`, { method: 'DELETE' })
}

export async function listConversations(): Promise<Conversation[]> {
  return api<Conversation[]>('/conversations')
}

export async function createConversation(botId: number, title: string): Promise<Conversation> {
  return api<Conversation>('/conversations', {
    method: 'POST',
    body: JSON.stringify({ bot_id: botId, title }),
  })
}

export async function getMessages(
  conversationId: number,
  before?: number,
  limit = 20,
): Promise<{ messages: Message[]; has_more: boolean }> {
  const params = new URLSearchParams({ limit: String(limit) })
  if (before) params.set('before', String(before))
  return api(`/conversations/${conversationId}/messages?${params}`)
}

export async function sendMessage(
  conversationId: number,
  layers: MessageLayers,
): Promise<Message> {
  return api<Message>('/messages/send', {
    method: 'POST',
    body: JSON.stringify({ conversation_id: conversationId, layers }),
  })
}
