/**
 * Typed fetch helpers and data types for the OaSis controller API.
 *
 * All requests are made client-side (no server runtime) using
 * `NEXT_PUBLIC_API_BASE_URL` as the base URL — empty string in production
 * so requests go to the same origin served by NGINX.
 *
 * See `aspec/architecture/apis.md` for the full API contract.
 */

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
  constructor(public readonly status: number, message: string) {
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

/** Fetches all registered apps and agents from the controller. */
export async function fetchApps(): Promise<AppsResponse> {
  return apiFetch<AppsResponse>('/api/v1/apps')
}

/** Fetches the current controller health snapshot. */
export async function fetchStatus(): Promise<Status> {
  return apiFetch<Status>('/api/v1/status')
}
