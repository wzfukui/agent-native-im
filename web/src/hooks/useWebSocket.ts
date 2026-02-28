import { useEffect, useRef, useCallback, useState } from 'react'
import type { Message, WSOutbound } from '../types'

interface UseWebSocketReturn {
  realtimeMessages: Message[]
  streamingMessages: Map<string, Message>
  connected: boolean
}

export function useWebSocket(token: string | null): UseWebSocketReturn {
  const [realtimeMessages, setRealtimeMessages] = useState<Message[]>([])
  const [streamingMessages, setStreamingMessages] = useState<Map<string, Message>>(new Map())
  const [connected, setConnected] = useState(false)
  const wsRef = useRef<WebSocket | null>(null)
  const retryRef = useRef(0)
  const timerRef = useRef<ReturnType<typeof setTimeout>>(undefined)

  const connect = useCallback(() => {
    if (!token) return

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const host = window.location.host
    const ws = new WebSocket(`${protocol}//${host}/api/v1/ws?token=${token}`)
    wsRef.current = ws

    ws.onopen = () => {
      setConnected(true)
      retryRef.current = 0
    }

    ws.onclose = () => {
      setConnected(false)
      wsRef.current = null
      // Exponential backoff reconnect
      const delay = Math.min(1000 * 2 ** retryRef.current, 30000)
      retryRef.current++
      timerRef.current = setTimeout(connect, delay)
    }

    ws.onerror = () => {
      ws.close()
    }

    ws.onmessage = (event) => {
      try {
        const msg: WSOutbound = JSON.parse(event.data)
        switch (msg.type) {
          case 'message.new': {
            const m = msg.data as Message
            setRealtimeMessages(prev => [...prev, m])
            // Remove streaming message if this is the final version
            if (m.stream_id) {
              setStreamingMessages(prev => {
                const next = new Map(prev)
                next.delete(m.stream_id!)
                return next
              })
            }
            break
          }
          case 'stream.start':
          case 'stream.delta': {
            const m = msg.data as Message
            if (m.stream_id) {
              setStreamingMessages(prev => {
                const next = new Map(prev)
                next.set(m.stream_id!, m)
                return next
              })
            }
            break
          }
          case 'stream.end': {
            const m = msg.data as Message
            if (m.stream_id) {
              setStreamingMessages(prev => {
                const next = new Map(prev)
                next.delete(m.stream_id!)
                return next
              })
            }
            // stream.end is persisted, will arrive as message.new too
            setRealtimeMessages(prev => [...prev, m])
            break
          }
          case 'pong':
            break
          case 'error':
            console.error('WS error:', msg.data)
            break
        }
      } catch {
        console.error('WS parse error:', event.data)
      }
    }
  }, [token])

  useEffect(() => {
    connect()
    return () => {
      clearTimeout(timerRef.current)
      wsRef.current?.close()
    }
  }, [connect])

  // Ping keepalive every 25s
  useEffect(() => {
    if (!connected) return
    const interval = setInterval(() => {
      wsRef.current?.send(JSON.stringify({ type: 'ping' }))
    }, 25000)
    return () => clearInterval(interval)
  }, [connected])

  return { realtimeMessages, streamingMessages, connected }
}
