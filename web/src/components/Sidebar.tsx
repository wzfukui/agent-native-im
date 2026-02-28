import { useState, useEffect, useCallback } from 'react'
import type { Bot, Conversation } from '../types'
import * as api from '../api'

interface Props {
  selectedConversationId: number | null
  onSelectConversation: (conv: Conversation) => void
  onLogout: () => void
}

export default function Sidebar({ selectedConversationId, onSelectConversation, onLogout }: Props) {
  const [bots, setBots] = useState<Bot[]>([])
  const [conversations, setConversations] = useState<Conversation[]>([])
  const [newBotName, setNewBotName] = useState('')
  const [newBotToken, setNewBotToken] = useState<string | null>(null)
  const [expandedBotId, setExpandedBotId] = useState<number | null>(null)
  const [newConvTitle, setNewConvTitle] = useState('')
  const [showNewBot, setShowNewBot] = useState(false)

  const loadData = useCallback(async () => {
    const [b, c] = await Promise.all([api.listBots(), api.listConversations()])
    setBots(b)
    setConversations(c)
  }, [])

  useEffect(() => { loadData() }, [loadData])

  const handleCreateBot = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!newBotName.trim()) return
    const bot = await api.createBot(newBotName.trim())
    setNewBotToken(bot.token ?? null)
    setNewBotName('')
    setShowNewBot(false)
    await loadData()
  }

  const handleCreateConv = async (botId: number) => {
    const title = newConvTitle.trim() || 'New conversation'
    const conv = await api.createConversation(botId, title)
    setNewConvTitle('')
    await loadData()
    onSelectConversation(conv)
  }

  const convsByBot = (botId: number) =>
    conversations.filter(c => c.bot_id === botId)

  return (
    <div className="w-64 bg-gray-900 text-gray-100 flex flex-col h-full">
      {/* Header */}
      <div className="p-3 border-b border-gray-700 flex items-center justify-between">
        <span className="font-bold text-sm">Agent-Native IM</span>
        <button onClick={onLogout} className="text-xs text-gray-400 hover:text-white">
          Logout
        </button>
      </div>

      {/* Bot token display */}
      {newBotToken && (
        <div className="m-2 p-2 bg-yellow-900/50 border border-yellow-700 rounded text-xs">
          <p className="font-bold text-yellow-300 mb-1">Bot Token (save now!)</p>
          <code className="break-all text-yellow-100 select-all">{newBotToken}</code>
          <button
            onClick={() => setNewBotToken(null)}
            className="mt-1 text-yellow-400 hover:text-yellow-200 text-xs underline block"
          >
            Dismiss
          </button>
        </div>
      )}

      {/* Bot list */}
      <div className="flex-1 overflow-y-auto">
        {bots.map(bot => (
          <div key={bot.id}>
            <button
              onClick={() => setExpandedBotId(expandedBotId === bot.id ? null : bot.id)}
              className={`w-full text-left px-3 py-2 text-sm hover:bg-gray-800 flex items-center gap-2 ${
                expandedBotId === bot.id ? 'bg-gray-800' : ''
              }`}
            >
              <span className="w-2 h-2 rounded-full bg-green-500 shrink-0" />
              <span className="truncate">{bot.name}</span>
              <span className="ml-auto text-gray-500 text-xs">
                {convsByBot(bot.id).length}
              </span>
            </button>

            {/* Conversations under this bot */}
            {expandedBotId === bot.id && (
              <div className="bg-gray-850">
                {convsByBot(bot.id).map(conv => (
                  <button
                    key={conv.id}
                    onClick={() => onSelectConversation(conv)}
                    className={`w-full text-left pl-8 pr-3 py-1.5 text-sm truncate hover:bg-gray-700 ${
                      selectedConversationId === conv.id
                        ? 'bg-gray-700 text-white'
                        : 'text-gray-300'
                    }`}
                  >
                    {conv.title}
                  </button>
                ))}
                {/* New conversation */}
                <div className="pl-8 pr-3 py-1.5 flex gap-1">
                  <input
                    value={newConvTitle}
                    onChange={e => setNewConvTitle(e.target.value)}
                    placeholder="New chat..."
                    className="flex-1 bg-gray-800 border border-gray-600 rounded px-2 py-1 text-xs text-gray-200 placeholder:text-gray-500 focus:outline-none"
                    onKeyDown={e => e.key === 'Enter' && handleCreateConv(bot.id)}
                  />
                  <button
                    onClick={() => handleCreateConv(bot.id)}
                    className="text-xs text-blue-400 hover:text-blue-300 px-1"
                  >
                    +
                  </button>
                </div>
              </div>
            )}
          </div>
        ))}

        {bots.length === 0 && (
          <p className="text-gray-500 text-xs px-3 py-4 text-center">
            No bots yet. Create one below.
          </p>
        )}
      </div>

      {/* Create bot */}
      <div className="border-t border-gray-700 p-3">
        {showNewBot ? (
          <form onSubmit={handleCreateBot} className="flex gap-1">
            <input
              value={newBotName}
              onChange={e => setNewBotName(e.target.value)}
              placeholder="Bot name"
              className="flex-1 bg-gray-800 border border-gray-600 rounded px-2 py-1 text-xs text-gray-200 placeholder:text-gray-500 focus:outline-none"
              autoFocus
            />
            <button type="submit" className="text-xs text-blue-400 hover:text-blue-300 px-2">
              Create
            </button>
            <button
              type="button"
              onClick={() => setShowNewBot(false)}
              className="text-xs text-gray-500 hover:text-gray-300"
            >
              Cancel
            </button>
          </form>
        ) : (
          <button
            onClick={() => setShowNewBot(true)}
            className="w-full text-sm text-gray-400 hover:text-white py-1"
          >
            + New Bot
          </button>
        )}
      </div>
    </div>
  )
}
