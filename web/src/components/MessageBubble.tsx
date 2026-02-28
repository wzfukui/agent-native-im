import { useState } from 'react'
import type { Message } from '../types'

interface Props {
  message: Message
  isStreaming?: boolean
}

export default function MessageBubble({ message, isStreaming }: Props) {
  const isUser = message.sender_type === 'user'
  const { layers } = message
  const [showThinking, setShowThinking] = useState(false)
  const [showData, setShowData] = useState(false)

  return (
    <div className={`flex ${isUser ? 'justify-end' : 'justify-start'} mb-3`}>
      <div
        className={`max-w-[75%] rounded-lg px-3 py-2 text-sm ${
          isUser
            ? 'bg-blue-600 text-white'
            : 'bg-gray-100 text-gray-900 border border-gray-200'
        }`}
      >
        {/* Status layer — progress bar */}
        {layers.status && (
          <div className="mb-2">
            <div className="flex items-center gap-2 text-xs opacity-75 mb-1">
              <span>{layers.status.phase}</span>
              {layers.status.text && <span>— {layers.status.text}</span>}
            </div>
            <div className={`w-full h-1.5 rounded-full ${isUser ? 'bg-blue-400' : 'bg-gray-300'}`}>
              <div
                className={`h-full rounded-full transition-all duration-300 ${
                  isUser ? 'bg-white' : 'bg-blue-500'
                } ${isStreaming ? 'animate-pulse' : ''}`}
                style={{ width: `${(layers.status.progress ?? 0) * 100}%` }}
              />
            </div>
          </div>
        )}

        {/* Summary — main content */}
        {layers.summary && (
          <p className="whitespace-pre-wrap">{layers.summary}</p>
        )}

        {/* Thinking layer — collapsible */}
        {layers.thinking && (
          <div className="mt-2">
            <button
              onClick={() => setShowThinking(!showThinking)}
              className={`text-xs ${isUser ? 'text-blue-200' : 'text-gray-500'} hover:underline`}
            >
              {showThinking ? 'Hide' : 'Show'} thinking
            </button>
            {showThinking && (
              <pre
                className={`mt-1 text-xs whitespace-pre-wrap rounded p-2 ${
                  isUser ? 'bg-blue-700/50' : 'bg-gray-50 text-gray-600'
                }`}
              >
                {layers.thinking}
              </pre>
            )}
          </div>
        )}

        {/* Data layer — collapsible JSON */}
        {layers.data && Object.keys(layers.data).length > 0 && (
          <div className="mt-2">
            <button
              onClick={() => setShowData(!showData)}
              className={`text-xs ${isUser ? 'text-blue-200' : 'text-gray-500'} hover:underline`}
            >
              {showData ? 'Hide' : 'Show'} data
            </button>
            {showData && (
              <pre
                className={`mt-1 text-xs whitespace-pre-wrap rounded p-2 overflow-x-auto ${
                  isUser ? 'bg-blue-700/50' : 'bg-gray-50 text-gray-600'
                }`}
              >
                {JSON.stringify(layers.data, null, 2)}
              </pre>
            )}
          </div>
        )}

        {/* Interaction layer — buttons */}
        {layers.interaction && (
          <div className="mt-2 pt-2 border-t border-current/10">
            <p className={`text-xs mb-2 ${isUser ? 'text-blue-200' : 'text-gray-600'}`}>
              {layers.interaction.prompt}
            </p>
            <div className="flex flex-wrap gap-1.5">
              {layers.interaction.options.map(opt => (
                <button
                  key={opt.value}
                  className={`text-xs px-3 py-1 rounded-full border ${
                    isUser
                      ? 'border-blue-300 text-blue-100 hover:bg-blue-500'
                      : 'border-gray-300 text-gray-700 hover:bg-gray-200'
                  }`}
                >
                  {opt.label}
                </button>
              ))}
            </div>
          </div>
        )}

        {/* Streaming indicator */}
        {isStreaming && (
          <div className="mt-1 flex items-center gap-1">
            <span className="w-1.5 h-1.5 rounded-full bg-current animate-pulse" />
            <span className="text-xs opacity-60">streaming...</span>
          </div>
        )}

        {/* Timestamp */}
        <div className={`text-xs mt-1 ${isUser ? 'text-blue-200' : 'text-gray-400'}`}>
          {new Date(message.created_at).toLocaleTimeString()}
        </div>
      </div>
    </div>
  )
}
