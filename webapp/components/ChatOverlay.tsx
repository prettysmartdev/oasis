'use client'

import { useEffect, useRef, useState } from 'react'
import { getChatHistory, sendChatMessage, ChatMessage } from '@/lib/api'

interface ChatOverlayProps {
  onClose: () => void
}

/**
 * Full-screen chat overlay that slides up from below.
 *
 * Fetches chat history on mount, supports optimistic user bubbles, and shows
 * an error alert if sending fails. Entry animation is skipped when
 * `prefers-reduced-motion` is set.
 */
export default function ChatOverlay({ onClose }: ChatOverlayProps) {
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [input, setInput] = useState('')
  const [sending, setSending] = useState(false)
  const [sendError, setSendError] = useState<string | null>(null)
  const [visible, setVisible] = useState(false)
  const threadRef = useRef<HTMLDivElement>(null)

  // Check reduced motion preference
  const prefersReducedMotion =
    typeof window !== 'undefined'
      ? window.matchMedia('(prefers-reduced-motion: reduce)').matches
      : false

  useEffect(() => {
    // Entry animation
    if (!prefersReducedMotion) {
      requestAnimationFrame(() => setVisible(true))
    } else {
      setVisible(true)
    }
    // Load history
    getChatHistory()
      .then((res) => setMessages(res.items))
      .catch(() => {})
  }, [prefersReducedMotion])

  useEffect(() => {
    // Scroll to bottom after each message change
    if (threadRef.current) {
      threadRef.current.scrollTo({ top: threadRef.current.scrollHeight })
    }
  }, [messages])

  const handleSend = async () => {
    const text = input.trim()
    if (!text || sending) return

    setSendError(null)
    setSending(true)
    setInput('')

    // Optimistic bubble
    const optimisticId = 'optimistic-' + Date.now()
    const optimisticMsg: ChatMessage = {
      id: optimisticId,
      role: 'user',
      content: text,
      createdAt: new Date().toISOString(),
    }
    setMessages((prev) => [...prev, optimisticMsg])

    try {
      const { userMessage, assistantMessage } = await sendChatMessage(text)
      setMessages((prev) => [
        ...prev.filter((m) => m.id !== optimisticId),
        userMessage,
        assistantMessage,
      ])
    } catch (err) {
      setMessages((prev) => prev.filter((m) => m.id !== optimisticId))
      setSendError(err instanceof Error ? err.message : 'Failed to send')
      setInput(text)
    } finally {
      setSending(false)
    }
  }

  return (
    <div
      data-testid="chat-overlay"
      className="fixed inset-0 z-50 flex flex-col notch-safe-top"
      style={{
        transform: prefersReducedMotion ? 'none' : visible ? 'translateY(0)' : 'translateY(100%)',
        transition: prefersReducedMotion ? 'none' : 'transform 0.3s ease',
      }}
    >
      {/* bg-slate-900 starts below the notch so the wallpaper shows through the safe-area strip */}
      <div className="flex flex-col flex-1 bg-slate-900 overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-slate-700">
        <span className="text-white font-medium">Chat</span>
        <button
          onClick={onClose}
          aria-label="Close chat"
          className="text-slate-400 hover:text-white"
        >
          ✕
        </button>
      </div>

      {/* Thread */}
      <div ref={threadRef} className="flex-1 overflow-y-auto p-4 space-y-3">
        {messages.length === 0 ? (
          <p className="text-center text-slate-500">No messages yet. Say hello!</p>
        ) : (
          messages.map((msg) => (
            <div
              key={msg.id}
              className={`flex ${msg.role === 'user' ? 'justify-end' : 'justify-start'}`}
            >
              <div
                className={`rounded-lg px-3 py-2 max-w-xs text-sm text-white ${
                  msg.role === 'user' ? 'bg-teal-500' : 'bg-slate-700'
                }`}
              >
                {msg.content}
                <p className="text-xs opacity-60 mt-1">
                  {new Date(msg.createdAt).toLocaleTimeString()}
                </p>
              </div>
            </div>
          ))
        )}
      </div>

      {/* Error */}
      {sendError && (
        <div role="alert" className="mx-4 mb-2 text-red-400 text-sm">
          {sendError}
        </div>
      )}

      {/* Input area */}
      <div className="px-4 py-3 border-t border-slate-700 flex gap-2">
        <input
          type="text"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') handleSend()
          }}
          placeholder="Type a message…"
          aria-label="Chat message"
          className="flex-1 rounded-lg bg-slate-800 border border-slate-600 text-white text-sm px-3 py-2 focus:outline-none"
          disabled={sending}
        />
        <button
          onClick={handleSend}
          disabled={sending}
          aria-label="Send message"
          className="rounded-lg bg-teal-500 text-white px-4 py-2 text-sm disabled:opacity-50"
        >
          {sending ? '…' : 'Send'}
        </button>
      </div>
      </div>
    </div>
  )
}
