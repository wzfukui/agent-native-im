import { useState, useEffect, useRef, useCallback } from 'react'
import type { Conversation, Message } from '../types'
import * as api from '../api'
import MessageBubble from './MessageBubble'
import MessageInput from './MessageInput'

interface Props {
  conversation: Conversation | null
  realtimeMessages: Message[]
  streamingMessages: Map<string, Message>
}

export default function ChatView({ conversation, realtimeMessages, streamingMessages }: Props) {
  const [messages, setMessages] = useState<Message[]>([])
  const [hasMore, setHasMore] = useState(false)
  const [loading, setLoading] = useState(false)
  const bottomRef = useRef<HTMLDivElement>(null)
  const containerRef = useRef<HTMLDivElement>(null)

  // Load messages when conversation changes
  useEffect(() => {
    if (!conversation) {
      setMessages([])
      return
    }
    setMessages([])
    setHasMore(false)
    setLoading(true)
    api.getMessages(conversation.id).then(res => {
      // API returns newest-first, we display oldest-first
      setMessages((res.messages ?? []).reverse())
      setHasMore(res.has_more)
      setLoading(false)
    }).catch(() => {
      setLoading(false)
    })
  }, [conversation?.id])

  // Merge realtime messages (deduplicate by id)
  const allMessages = useCallback(() => {
    const map = new Map<number, Message>()
    for (const m of messages) map.set(m.id, m)
    for (const m of realtimeMessages) {
      if (m.conversation_id === conversation?.id) {
        map.set(m.id, m)
      }
    }
    const sorted = Array.from(map.values()).sort((a, b) => a.id - b.id)

    // Append streaming messages at the end
    const streams: Message[] = []
    streamingMessages.forEach((m, streamId) => {
      if (m.conversation_id === conversation?.id) {
        streams.push({ ...m, id: -1, stream_id: streamId })
      }
    })

    return [...sorted, ...streams]
  }, [messages, realtimeMessages, streamingMessages, conversation?.id])

  // Auto scroll to bottom on new messages
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [realtimeMessages, streamingMessages])

  // Load more (older messages)
  const loadMore = async () => {
    if (!conversation || !hasMore || loading) return
    const oldest = messages[0]
    if (!oldest) return
    setLoading(true)
    const res = await api.getMessages(conversation.id, oldest.id)
    setMessages(prev => [...res.messages.reverse(), ...prev])
    setHasMore(res.has_more)
    setLoading(false)
  }

  const handleSend = async (text: string) => {
    if (!conversation) return
    await api.sendMessage(conversation.id, { summary: text })
  }

  if (!conversation) {
    return (
      <div className="flex-1 flex items-center justify-center text-gray-400 text-sm">
        Select a conversation to start chatting
      </div>
    )
  }

  const displayed = allMessages()

  return (
    <div className="flex-1 flex flex-col h-full">
      {/* Header */}
      <div className="border-b border-gray-200 px-4 py-3 flex items-center gap-2">
        <h2 className="font-medium text-sm text-gray-900">{conversation.title}</h2>
        {conversation.bot && (
          <span className="text-xs text-gray-500 bg-gray-100 px-2 py-0.5 rounded">
            {conversation.bot.name}
          </span>
        )}
      </div>

      {/* Messages */}
      <div ref={containerRef} className="flex-1 overflow-y-auto px-4 py-3">
        {hasMore && (
          <button
            onClick={loadMore}
            disabled={loading}
            className="w-full text-center text-xs text-blue-500 hover:text-blue-700 py-2 disabled:opacity-50"
          >
            {loading ? 'Loading...' : 'Load older messages'}
          </button>
        )}

        {displayed.length === 0 && !loading && (
          <p className="text-center text-gray-400 text-sm py-8">
            No messages yet. Send the first one!
          </p>
        )}

        {displayed.map((msg, i) => (
          <MessageBubble
            key={msg.id > 0 ? msg.id : `stream-${msg.stream_id}-${i}`}
            message={msg}
            isStreaming={msg.id < 0}
          />
        ))}
        <div ref={bottomRef} />
      </div>

      {/* Input */}
      <MessageInput onSend={handleSend} />
    </div>
  )
}
