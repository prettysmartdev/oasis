'use client'

/**
 * AgentWindow — full-screen modal overlay for viewing agent run output.
 *
 * For tap-triggered agents it immediately fires a new run on mount and polls
 * until the run reaches a terminal status or a 30-second timeout elapses.
 * For schedule- or webhook-triggered agents it fetches the most recent run.
 *
 * Output is rendered according to the agent's `outputFmt`:
 *   - markdown  → react-markdown with remark-gfm (prose-invert styled)
 *   - html      → sandboxed <iframe srcdoc> (sandbox="allow-scripts")
 *   - plaintext → <pre> with Geist Mono font
 */

import { useEffect, useState, useCallback, useRef } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import {
  Agent,
  AgentRun,
  ApiError,
  fetchAgentRun,
  fetchLatestAgentRun,
  triggerAgentRun,
} from '@/lib/api'

interface AgentWindowProps {
  agent: Agent
  onClose: () => void
}

const POLL_INTERVAL_MS = 1000
const TIMEOUT_MS = 30_000
const CHECK_AGAIN_COOLDOWN_MS = 5000

function TriggerBadge({ trigger }: { trigger: Agent['trigger'] }) {
  const labels: Record<Agent['trigger'], string> = {
    tap: 'Tap',
    schedule: 'Schedule',
    webhook: 'Webhook',
  }
  const colours: Record<Agent['trigger'], string> = {
    tap: 'bg-teal-500/20 text-teal-300 border-teal-500/30',
    schedule: 'bg-amber-500/20 text-amber-300 border-amber-500/30',
    webhook: 'bg-purple-500/20 text-purple-300 border-purple-500/30',
  }
  return (
    <span
      className={`inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-medium ${colours[trigger]}`}
    >
      {labels[trigger]}
    </span>
  )
}

function Spinner() {
  return (
    <svg
      className="animate-spin h-8 w-8 text-teal-400"
      xmlns="http://www.w3.org/2000/svg"
      fill="none"
      viewBox="0 0 24 24"
      aria-hidden="true"
    >
      <circle
        className="opacity-25"
        cx="12"
        cy="12"
        r="10"
        stroke="currentColor"
        strokeWidth="4"
      />
      <path
        className="opacity-75"
        fill="currentColor"
        d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
      />
    </svg>
  )
}

function isUrl(icon: string): boolean {
  return icon.startsWith('http://') || icon.startsWith('https://')
}

function formatTimestamp(ts: string | null | undefined): string {
  if (!ts) return 'No runs yet'
  try {
    return new Intl.DateTimeFormat(undefined, {
      dateStyle: 'medium',
      timeStyle: 'short',
    }).format(new Date(ts))
  } catch {
    return ts
  }
}

function MarkdownOutput({ content }: { content: string }) {
  return (
    <div className="prose prose-invert prose-sm max-w-none prose-headings:text-white prose-p:text-slate-300 prose-strong:text-white prose-code:text-teal-300 prose-pre:bg-slate-900 prose-blockquote:border-teal-500 prose-blockquote:text-slate-300 prose-li:text-slate-300 prose-a:text-teal-400">
      <ReactMarkdown remarkPlugins={[remarkGfm]}>{content}</ReactMarkdown>
    </div>
  )
}

function HtmlOutput({ content }: { content: string }) {
  return (
    <iframe
      srcDoc={content}
      sandbox="allow-scripts"
      title="Agent HTML output"
      className="w-full min-h-[400px] border-0 rounded-lg bg-white"
      style={{ colorScheme: 'light' }}
    />
  )
}

function PlaintextOutput({ content }: { content: string }) {
  return (
    <pre className="font-mono text-sm text-slate-200 bg-slate-900 rounded-lg p-4 overflow-x-auto whitespace-pre-wrap break-words leading-relaxed">
      {content}
    </pre>
  )
}

function RunOutput({ run, outputFmt }: { run: AgentRun; outputFmt: Agent['outputFmt'] }) {
  if (!run.output) {
    return <p className="text-slate-500 text-sm italic">No output produced.</p>
  }
  switch (outputFmt) {
    case 'markdown':
      return <MarkdownOutput content={run.output} />
    case 'html':
      return <HtmlOutput content={run.output} />
    case 'plaintext':
      return <PlaintextOutput content={run.output} />
    default:
      return <PlaintextOutput content={run.output} />
  }
}

export function AgentWindow({ agent, onClose }: AgentWindowProps) {
  const [run, setRun] = useState<AgentRun | null>(null)
  const [loading, setLoading] = useState(true)
  const [timedOut, setTimedOut] = useState(false)
  const [checkAgainDisabled, setCheckAgainDisabled] = useState(false)
  const [errorMsg, setErrorMsg] = useState<string | null>(null)
  const pollingRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const startTimeRef = useRef<number>(Date.now())
  const [isVisible, setIsVisible] = useState(false)

  const stopPolling = useCallback(() => {
    if (pollingRef.current !== null) {
      clearInterval(pollingRef.current)
      pollingRef.current = null
    }
    if (timeoutRef.current !== null) {
      clearTimeout(timeoutRef.current)
      timeoutRef.current = null
    }
  }, [])

  const pollRun = useCallback(
    (runId: string) => {
      pollingRef.current = setInterval(async () => {
        try {
          const updated = await fetchAgentRun(runId)
          setRun(updated)
          if (updated.status !== 'running') {
            stopPolling()
            setLoading(false)
          }
        } catch {
          // Leave loading state; let timeout handle it
        }
      }, POLL_INTERVAL_MS)

      timeoutRef.current = setTimeout(() => {
        stopPolling()
        setTimedOut(true)
        setLoading(false)
      }, TIMEOUT_MS)
    },
    [stopPolling]
  )

  const startTapRun = useCallback(async () => {
    setLoading(true)
    setTimedOut(false)
    setErrorMsg(null)
    startTimeRef.current = Date.now()
    try {
      const { runId } = await triggerAgentRun(agent.slug)
      // Fetch initial run state immediately
      const initialRun = await fetchAgentRun(runId)
      setRun(initialRun)
      if (initialRun.status === 'running') {
        pollRun(runId)
      } else {
        setLoading(false)
      }
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        // RUN_IN_PROGRESS — poll the existing run
        const existingRunId = (err.body as { runId?: string } | undefined)?.runId
        if (existingRunId) {
          const existingRun = await fetchAgentRun(existingRunId).catch(() => null)
          if (existingRun) {
            setRun(existingRun)
            if (existingRun.status === 'running') {
              pollRun(existingRunId)
            } else {
              setLoading(false)
            }
            return
          }
        }
      }
      setErrorMsg(err instanceof Error ? err.message : 'Failed to start agent run.')
      setLoading(false)
    }
  }, [agent.slug, pollRun])

  const loadLatestRun = useCallback(async () => {
    setLoading(true)
    setErrorMsg(null)
    try {
      const latestRun = await fetchLatestAgentRun(agent.slug)
      setRun(latestRun)
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) {
        // No runs yet — that's fine
        setRun(null)
      } else {
        setErrorMsg(err instanceof Error ? err.message : 'Failed to load run.')
      }
    } finally {
      setLoading(false)
    }
  }, [agent.slug])

  // Animate open
  useEffect(() => {
    const frame = requestAnimationFrame(() => setIsVisible(true))
    return () => cancelAnimationFrame(frame)
  }, [])

  useEffect(() => {
    if (agent.trigger === 'tap') {
      startTapRun()
    } else {
      loadLatestRun()
    }
    return () => stopPolling()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // Keyboard: Escape closes the window
  useEffect(() => {
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', handleKey)
    return () => document.removeEventListener('keydown', handleKey)
  }, [onClose])

  const handleCheckAgain = useCallback(() => {
    setCheckAgainDisabled(true)
    setTimedOut(false)
    startTapRun()
    setTimeout(() => setCheckAgainDisabled(false), CHECK_AGAIN_COOLDOWN_MS)
  }, [startTapRun])

  const handleClose = useCallback(() => {
    setIsVisible(false)
    // Allow CSS transition to play before unmounting
    setTimeout(onClose, 200)
  }, [onClose])

  const runTimestamp =
    run?.finishedAt ?? run?.startedAt ?? null

  return (
    <>
      {/* Backdrop */}
      <div
        aria-hidden="true"
        onClick={handleClose}
        className="fixed inset-0 z-40 bg-black/60 backdrop-blur-sm transition-opacity duration-200 motion-reduce:transition-none"
        style={{ opacity: isVisible ? 1 : 0 }}
      />

      {/* Modal panel */}
      <div
        role="dialog"
        aria-modal="true"
        aria-label={`Agent: ${agent.name}`}
        className="fixed inset-0 z-50 flex items-center justify-center p-4 pointer-events-none"
      >
        <div
          className="pointer-events-auto w-full max-w-2xl max-h-[90vh] flex flex-col rounded-2xl bg-slate-900 border border-slate-700 shadow-2xl transition-all duration-200 motion-reduce:transition-none overflow-hidden"
          style={{
            opacity: isVisible ? 1 : 0,
            transform: isVisible ? 'translateY(0) scale(1)' : 'translateY(16px) scale(0.97)',
          }}
        >
          {/* Header */}
          <header className="flex items-start gap-4 p-6 border-b border-slate-700 shrink-0">
            <div className="w-14 h-14 rounded-xl bg-slate-800 border border-slate-700 flex items-center justify-center shrink-0 overflow-hidden">
              {isUrl(agent.icon) ? (
                <img
                  src={agent.icon}
                  alt=""
                  aria-hidden="true"
                  className="w-10 h-10 object-contain"
                />
              ) : (
                <span className="text-3xl" role="img" aria-hidden="true">
                  {agent.icon || '🤖'}
                </span>
              )}
            </div>

            <div className="flex-1 min-w-0">
              <h2 className="text-lg font-bold text-white leading-tight truncate">
                {agent.name}
              </h2>
              {agent.description && (
                <p className="text-sm text-slate-400 mt-0.5 line-clamp-2">
                  {agent.description}
                </p>
              )}
              <div className="flex items-center gap-3 mt-2">
                <TriggerBadge trigger={agent.trigger} />
                <span className="text-xs text-slate-500 font-mono">
                  {loading && agent.trigger === 'tap'
                    ? 'Running…'
                    : formatTimestamp(runTimestamp)}
                </span>
              </div>
            </div>

            <button
              onClick={handleClose}
              aria-label="Close agent window"
              className="shrink-0 p-2 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-teal-400 min-w-[44px] min-h-[44px] flex items-center justify-center"
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                width="20"
                height="20"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
                aria-hidden="true"
              >
                <path d="M18 6L6 18M6 6l12 12" />
              </svg>
            </button>
          </header>

          {/* Body */}
          <div className="flex-1 overflow-y-auto p-6">
            {/* Loading / running state */}
            {loading && (
              <div
                role="status"
                aria-live="polite"
                className="flex flex-col items-center justify-center gap-4 py-16"
              >
                <Spinner />
                <p className="text-slate-400 text-sm animate-pulse motion-reduce:animate-none">
                  Agent running…
                </p>
              </div>
            )}

            {/* Timed-out state */}
            {!loading && timedOut && (
              <div
                role="status"
                aria-live="polite"
                className="flex flex-col items-center justify-center gap-4 py-16 text-center"
              >
                <span className="text-4xl" aria-hidden="true">
                  ⏱
                </span>
                <p className="text-slate-300 text-sm max-w-sm">
                  Agent is taking longer than expected — check back later.
                </p>
                <button
                  onClick={handleCheckAgain}
                  disabled={checkAgainDisabled}
                  aria-disabled={checkAgainDisabled}
                  className="mt-2 px-4 py-2 rounded-lg bg-teal-600 text-white text-sm font-medium transition-colors hover:bg-teal-500 disabled:opacity-40 disabled:cursor-not-allowed focus:outline-none focus-visible:ring-2 focus-visible:ring-teal-400 min-h-[44px]"
                >
                  {checkAgainDisabled ? 'Please wait…' : 'Check again'}
                </button>
              </div>
            )}

            {/* Error state (API/network error) */}
            {!loading && !timedOut && errorMsg && (
              <div
                role="alert"
                className="flex flex-col items-center justify-center gap-3 py-16 text-center"
              >
                <span className="text-4xl" aria-hidden="true">
                  ⚠️
                </span>
                <p className="text-red-400 text-sm font-medium">Agent run failed.</p>
                <p className="text-slate-500 text-xs max-w-sm">{errorMsg}</p>
              </div>
            )}

            {/* Run error status */}
            {!loading && !timedOut && !errorMsg && run?.status === 'error' && (
              <div
                role="alert"
                className="flex flex-col items-center justify-center gap-3 py-16 text-center"
              >
                <span className="text-4xl" aria-hidden="true">
                  ⚠️
                </span>
                <p className="text-red-400 text-sm font-medium">Agent run failed.</p>
                {run.output && (
                  <p className="text-slate-500 text-xs max-w-sm">{run.output}</p>
                )}
              </div>
            )}

            {/* No runs yet (schedule/webhook agents) */}
            {!loading && !timedOut && !errorMsg && !run && agent.trigger !== 'tap' && (
              <div
                className="flex flex-col items-center justify-center gap-3 py-16 text-center"
                aria-label="No runs yet"
              >
                <span className="text-4xl" aria-hidden="true">
                  🕐
                </span>
                <p className="text-slate-400 text-sm">
                  No runs yet — this agent hasn&apos;t been triggered yet.
                </p>
                {agent.trigger === 'schedule' && agent.schedule && (
                  <p className="text-slate-500 text-xs">
                    Schedule:{' '}
                    <span className="font-mono text-slate-400">{agent.schedule}</span>
                  </p>
                )}
                {agent.trigger === 'webhook' && (
                  <p className="text-slate-500 text-xs">
                    Trigger this agent via its webhook endpoint.
                  </p>
                )}
              </div>
            )}

            {/* Successful run output */}
            {!loading && !timedOut && !errorMsg && run && run.status !== 'error' && (
              <RunOutput run={run} outputFmt={agent.outputFmt} />
            )}
          </div>
        </div>
      </div>
    </>
  )
}
