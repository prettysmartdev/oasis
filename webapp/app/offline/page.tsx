'use client'

/**
 * Offline fallback page — rendered by the service worker when the network
 * is unavailable and no cached document exists for the requested URL.
 *
 * This is a pure static page with no data fetching.
 */
export default function OfflinePage() {
  return (
    <div className="min-h-screen bg-slate-900 flex flex-col items-center justify-center gap-6 px-6 text-center">
      {/* Animated wifi-off icon — respects prefers-reduced-motion via globals.css */}
      <div className="offline-icon-animate">
        <svg
          xmlns="http://www.w3.org/2000/svg"
          width="80"
          height="80"
          viewBox="0 0 24 24"
          fill="none"
          stroke="#2DD4BF"
          strokeWidth="1.5"
          strokeLinecap="round"
          strokeLinejoin="round"
          aria-hidden="true"
        >
          <line x1="1" y1="1" x2="23" y2="23" />
          <path d="M16.72 11.06A10.94 10.94 0 0 1 19 12.55" />
          <path d="M5 12.55a10.94 10.94 0 0 1 5.17-2.39" />
          <path d="M10.71 5.05A16 16 0 0 1 22.56 9" />
          <path d="M1.42 9a15.91 15.91 0 0 1 4.7-2.88" />
          <path d="M8.53 16.11a6 6 0 0 1 6.95 0" />
          <circle cx="12" cy="20" r="1" fill="#2DD4BF" stroke="none" />
        </svg>
      </div>

      <div className="space-y-2">
        <h1 className="text-2xl font-semibold text-white">You&apos;re offline</h1>
        <p className="text-slate-400 max-w-xs">
          OaSis couldn&apos;t reach your tailnet. Reconnect and try again.
        </p>
      </div>
    </div>
  )
}
