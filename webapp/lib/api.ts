/**
 * Typed fetch helpers and data types for the OaSis controller API.
 *
 * All requests are made client-side (no server runtime) using
 * `NEXT_PUBLIC_API_BASE_URL` as the base URL — empty string in production
 * so requests go to the same origin served by NGINX.
 *
 * See `aspec/architecture/apis.md` for the full API contract.
 */

/** An agent trigger type. */
export type AgentTrigger = 'tap' | 'schedule' | 'webhook'

/** An agent output format. */
export type AgentOutputFmt = 'markdown' | 'html' | 'plaintext'

/** An agent run status. */
export type AgentRunStatus = 'running' | 'done' | 'error'

/**
 * A registered agent as returned by GET /api/v1/agents.
 */
export interface Agent {
  id: string
  name: string
  slug: string
  description: string
  icon: string
  prompt: string
  trigger: AgentTrigger
  schedule: string
  outputFmt: AgentOutputFmt
  enabled: boolean
  createdAt: string
  updatedAt: string
}

/** An agent run as returned by GET /api/v1/agents/runs/:runId */
export interface AgentRun {
  id: string
  agentId: string
  triggerSrc: string
  status: AgentRunStatus
  output: string
  startedAt: string
  finishedAt: string | null
}

/** Paginated list envelope returned by GET /api/v1/agents */
export interface AgentsResponse {
  items: Agent[]
  total: number
}

/** Response from triggering a new agent run. */
export interface TriggerRunResponse {
  runId: string
}

/** Health status reported by the controller for a registered app or agent. */
export type AppHealth = 'healthy' | 'unreachable' | 'unknown'

/**
 * A registered app or agent as returned by `GET /api/v1/apps`.
 *
 * Items tagged with `"agent"` are displayed on the Agents page of the
 * dashboard; all other items appear on the Apps page.
 *
 * The `icon` field is either a plain emoji string (e.g. `"🤖"`) or an
 * absolute HTTPS image URL. Components must handle both forms.
 */
export interface App {
  id: string
  name: string
  slug: string
  upstreamURL: string
  displayName: string
  description: string
  icon: string
  tags: string[]
  enabled: boolean
  health: AppHealth
  accessType: 'direct' | 'proxy'
  createdAt: string
  updatedAt: string
}

/**
 * Controller health snapshot returned by `GET /api/v1/status`.
 * Used by `BottomNav` to drive the system status indicator.
 */
export interface Status {
  tailscaleConnected: boolean
  tailscaleIP: string
  tailscaleHostname: string
  nginxStatus: 'running' | 'stopped' | 'error'
  registeredAppCount: number
  version: string
}

/** A single turn in the persistent chat conversation. */
export interface ChatMessage {
  id: string
  role: 'user' | 'assistant'
  content: string
  createdAt: string
}

/** Response from POST /api/v1/chat/messages. */
export interface ChatResponse {
  userMessage: ChatMessage
  assistantMessage: ChatMessage
}

/** Paginated chat history from GET /api/v1/chat/messages. */
export interface ChatHistoryResponse {
  items: ChatMessage[]
  total: number
}

/** Paginated list envelope returned by `GET /api/v1/apps`. */
export interface AppsResponse {
  items: App[]
  total: number
}

/**
 * Thrown by API helpers when the controller responds with a non-2xx status or
 * when a network error occurs and is caught by the caller.
 *
 * `status` is the HTTP status code (0 for network-level failures where no
 * response was received).
 */
export class ApiError extends Error {
  constructor(
    public readonly status: number,
    message: string,
    /** Raw parsed body from the error response, if available. */
    public readonly body?: Record<string, unknown>
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

const BASE_URL = process.env.NEXT_PUBLIC_API_BASE_URL ?? ''

async function apiFetch<T>(path: string): Promise<T> {
  const res = await fetch(`${BASE_URL}${path}`)
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }))
    throw new ApiError(res.status, body.error ?? res.statusText)
  }
  return res.json() as Promise<T>
}

async function apiPost<T>(path: string, body?: unknown): Promise<T> {
  const res = await fetch(`${BASE_URL}${path}`, {
    method: 'POST',
    headers: body ? { 'Content-Type': 'application/json' } : {},
    body: body ? JSON.stringify(body) : undefined,
  })
  if (!res.ok) {
    const bodyData = await res.json().catch(() => ({ error: res.statusText }))
    throw new ApiError(res.status, bodyData.error ?? res.statusText, bodyData)
  }
  if (res.status === 204 || res.headers.get('content-length') === '0') {
    return undefined as T
  }
  return res.json() as Promise<T>
}

/** Fetches all registered apps and agents from the controller. */
export async function fetchApps(): Promise<AppsResponse> {
  return apiFetch<AppsResponse>('/api/v1/apps')
}

/** Fetches the current controller health snapshot. */
export async function fetchStatus(): Promise<Status> {
  return apiFetch<Status>('/api/v1/status')
}

/** Fetches all registered agents from the controller. */
export async function fetchAgents(): Promise<AgentsResponse> {
  return apiFetch<AgentsResponse>('/api/v1/agents')
}

/** Triggers an agent tap-run. Returns the run ID. */
export async function triggerAgentRun(slug: string): Promise<TriggerRunResponse> {
  return apiPost<TriggerRunResponse>(`/api/v1/agents/${slug}/run`)
}

/** Polls for a specific agent run by ID. */
export async function fetchAgentRun(runId: string): Promise<AgentRun> {
  return apiFetch<AgentRun>(`/api/v1/agents/runs/${runId}`)
}

/** Fetches the latest agent run for a given agent slug. */
export async function fetchLatestAgentRun(slug: string): Promise<AgentRun> {
  return apiFetch<AgentRun>(`/api/v1/agents/${slug}/runs/latest`)
}

/** Fetches the full chat history. */
export async function getChatHistory(): Promise<ChatHistoryResponse> {
  return apiFetch<ChatHistoryResponse>('/api/v1/chat/messages')
}

/** Sends a message and returns both the user and assistant messages. */
export async function sendChatMessage(message: string): Promise<ChatResponse> {
  return apiPost<ChatResponse>('/api/v1/chat/messages', { message })
}
