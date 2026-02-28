import { useState } from 'react'
import type { Conversation } from './types'
import { useAuth } from './hooks/useAuth'
import { useWebSocket } from './hooks/useWebSocket'
import LoginPage from './components/LoginPage'
import Sidebar from './components/Sidebar'
import ChatView from './components/ChatView'

export default function App() {
  const { token, login, logout } = useAuth()
  const { realtimeMessages, streamingMessages, connected } = useWebSocket(token)
  const [selectedConversation, setSelectedConversation] = useState<Conversation | null>(null)

  if (!token) {
    return <LoginPage onLogin={login} />
  }

  return (
    <div className="h-screen flex">
      <Sidebar
        selectedConversationId={selectedConversation?.id ?? null}
        onSelectConversation={setSelectedConversation}
        onLogout={logout}
      />
      <div className="flex-1 flex flex-col">
        {/* Connection indicator */}
        <div className={`h-0.5 ${connected ? 'bg-green-500' : 'bg-red-500'}`} />
        <ChatView
          conversation={selectedConversation}
          realtimeMessages={realtimeMessages}
          streamingMessages={streamingMessages}
        />
      </div>
    </div>
  )
}
