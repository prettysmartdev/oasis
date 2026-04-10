'use client'

import { useEffect, useRef, useState } from 'react'
import { useReducedMotion } from 'framer-motion'
import { fetchStatus, type Status } from '@/lib/api'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog'

function StatusDot({ status }: { status: Status | null }) {
  if (!status) {
    return (
      <span className="absolute -top-0.5 -right-0.5 h-2.5 w-2.5 rounded-full bg-gray-400 ring-1 ring-slate-900" />
    )
  }

  const isGreen = status.tailscaleConnected && status.nginxStatus === 'running'
  const isRed =
    !status.tailscaleConnected ||
    status.nginxStatus === 'error' ||
    status.nginxStatus === 'stopped'

  if (isGreen) {
    return (
      <span className="absolute -top-0.5 -right-0.5 h-2.5 w-2.5 rounded-full bg-green-400 animate-pulse ring-1 ring-slate-900" />
    )
  }
  if (isRed) {
    return (
      <span className="absolute -top-0.5 -right-0.5 h-2.5 w-2.5 rounded-full bg-red-500 ring-1 ring-slate-900" />
    )
  }
  return (
    <span className="absolute -top-0.5 -right-0.5 h-2.5 w-2.5 rounded-full bg-amber-400 ring-1 ring-slate-900" />
  )
}

interface BottomNavProps {
  /** True when a proxy app iFrame is currently displayed. */
  appOpen?: boolean
  /** Called to close the iFrame and return to the homescreen. */
  onCloseApp?: () => void
  /** Called when the chat input receives focus — used to open the full ChatOverlay. */
  onChatOpen?: () => void
}

/**
 * Floating bottom navigation bar with two controls:
 *
 * - **Logo button** (bottom-left): expands a chat input field. The input is a
 *   placeholder for a future chatbot feature — submission does nothing yet.
 * - **Settings button** (bottom-right): opens a dialog showing controller
 *   health (hostname, Tailscale IP, NGINX status, app count, version).
 *   Hidden when a proxy app is open.
 *
 * When `appOpen` is true and the chat input is expanded, a home button slides
 * in from the right to allow closing the iFrame.
 *
 * The settings button carries a status dot derived from `GET /api/v1/status`,
 * polled every 30 seconds. Gray = no data, green = fully healthy,
 * amber = degraded, red = error/disconnected.
 */
export default function BottomNav({ appOpen, onCloseApp, onChatOpen }: BottomNavProps) {
  const prefersReducedMotion = useReducedMotion()
  const [status, setStatus] = useState<Status | null>(null)
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [chatOpen, setChatOpen] = useState(false)
  const chatInputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    const loadStatus = () => {
      fetchStatus().then(setStatus).catch(() => {
        // Status fetch failure is non-fatal; indicator stays gray
      })
    }

    loadStatus()
    const interval = setInterval(loadStatus, 30_000)
    return () => clearInterval(interval)
  }, [])

  return (
    <>
      <nav
        aria-label="Bottom navigation"
        className="fixed bottom-0 inset-x-0 z-40 flex items-end justify-between px-6 pb-8 pointer-events-none bottom-nav-safe"
      >
        {/* Logo / chat button */}
        <div className="pointer-events-auto flex items-end gap-3">
          <button
            onClick={() => setChatOpen((v) => !v)}
            aria-label="Open chat"
            aria-expanded={chatOpen}
            className="w-12 h-12 rounded-full flex items-center justify-center text-2xl shadow-lg focus:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-transparent"
            style={{
              background:
                'linear-gradient(135deg, #2DD4BF, #7c3aed, #F59E0B, #2DD4BF)',
              backgroundSize: '300% 300%',
              animation: prefersReducedMotion ? undefined : 'iridescent 6s ease infinite',
            }}
          >
            🌴
          </button>

          {/* Chat input — slides in when open */}
          <div
            className="overflow-hidden transition-all duration-300"
            style={{ width: chatOpen ? '220px' : '0px', opacity: chatOpen ? 1 : 0 }}
          >
            <input
              ref={chatInputRef}
              type="text"
              placeholder="Ask OaSis…"
              aria-label="Chat with OaSis"
              className="w-full rounded-full bg-slate-900/90 border border-slate-700 text-white text-sm px-4 py-2.5 focus:outline-none focus:ring-2 focus:ring-primary placeholder:text-slate-500"
              onFocus={onChatOpen}
              onKeyDown={(e) => {
                if (e.key === 'Escape') setChatOpen(false)
              }}
            />
          </div>

        </div>

        {/* Right side: settings button (hidden when proxy app open) or home button */}
        <div className="pointer-events-auto relative flex items-end">
          {/* Settings button — hidden when a proxy app is open */}
          {!appOpen && (
            <>
              <button
                onClick={() => setSettingsOpen(true)}
                aria-label="Open settings"
                className="w-12 h-12 rounded-full bg-slate-800/90 border border-slate-700 flex items-center justify-center text-2xl shadow-lg focus:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-transparent"
              >
                ⚙️
              </button>
              <StatusDot status={status} />
            </>
          )}

          {/* Home button — slides in from the right when chat is open and a proxy app is displayed */}
          <div
            className="overflow-hidden transition-all duration-300"
            style={{
              width: (chatOpen && appOpen) ? '48px' : '0px',
              opacity: (chatOpen && appOpen) ? 1 : 0,
            }}
          >
            <button
              onClick={onCloseApp}
              aria-label="Return to home screen"
              className="w-12 h-12 rounded-full bg-slate-800/90 border border-slate-700 flex items-center justify-center text-2xl shadow-lg focus:outline-none focus-visible:ring-2 focus-visible:ring-primary"
            >
              🏠
            </button>
          </div>
        </div>
      </nav>

      <Dialog open={settingsOpen} onOpenChange={setSettingsOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>OaSis Status</DialogTitle>
            <DialogDescription>System information and connection status</DialogDescription>
          </DialogHeader>

          {status ? (
            <dl className="space-y-3 text-sm">
              <div className="flex justify-between items-center py-2 border-b border-slate-700">
                <dt className="text-slate-400">Hostname</dt>
                <dd className="font-mono text-primary">{status.tailscaleHostname}</dd>
              </div>
              <div className="flex justify-between items-center py-2 border-b border-slate-700">
                <dt className="text-slate-400">Tailscale IP</dt>
                <dd className="font-mono text-white">{status.tailscaleIP || '—'}</dd>
              </div>
              <div className="flex justify-between items-center py-2 border-b border-slate-700">
                <dt className="text-slate-400">Tailscale</dt>
                <dd className={status.tailscaleConnected ? 'text-green-400' : 'text-red-400'}>
                  {status.tailscaleConnected ? 'Connected' : 'Disconnected'}
                </dd>
              </div>
              <div className="flex justify-between items-center py-2 border-b border-slate-700">
                <dt className="text-slate-400">NGINX</dt>
                <dd
                  className={
                    status.nginxStatus === 'running'
                      ? 'text-green-400'
                      : status.nginxStatus === 'error'
                        ? 'text-red-400'
                        : 'text-amber-400'
                  }
                >
                  {status.nginxStatus === 'running' ? 'Running' : status.nginxStatus === 'error' ? 'Error' : status.nginxStatus === 'stopped' ? 'Stopped' : status.nginxStatus}
                </dd>
              </div>
              <div className="flex justify-between items-center py-2 border-b border-slate-700">
                <dt className="text-slate-400">Registered apps</dt>
                <dd className="font-mono text-white">{status.registeredAppCount}</dd>
              </div>
              <div className="flex justify-between items-center py-2">
                <dt className="text-slate-400">Version</dt>
                <dd className="font-mono text-slate-300 text-xs">{status.version}</dd>
              </div>
            </dl>
          ) : (
            <p className="text-slate-400 text-sm">Unable to reach the OaSis controller.</p>
          )}
        </DialogContent>
      </Dialog>
    </>
  )
}
