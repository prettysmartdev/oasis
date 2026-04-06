'use client'

import { useEffect, useState } from 'react'
import { fetchApps, type App, ApiError } from '@/lib/api'
import HomescreenLayout from '@/components/HomescreenLayout'
import BottomNav from '@/components/BottomNav'
import TimeOfDayBackground from '@/components/TimeOfDayBackground'

export default function HomePage() {
  const [agents, setAgents] = useState<App[]>([])
  const [apps, setApps] = useState<App[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetchApps()
      .then(({ items }) => {
        // Split by tags: items with tag 'agent' go to agents, rest to apps
        setAgents(items.filter((a) => a.tags.includes('agent')))
        setApps(items.filter((a) => !a.tags.includes('agent')))
      })
      .catch((err) => {
        setError(
          err instanceof ApiError ? err.message : 'Cannot reach OaSis controller'
        )
      })
      .finally(() => setLoading(false))
  }, [])

  return (
    <>
      <TimeOfDayBackground />
      {error && (
        <div
          role="alert"
          className="fixed top-0 inset-x-0 z-50 bg-red-600 text-white text-sm text-center py-2 px-4"
        >
          {error}
        </div>
      )}
      {loading ? (
        <div className="flex flex-col h-screen overflow-hidden" aria-busy="true" aria-label="Loading apps">
          {/* Skeleton header */}
          <header className="flex items-end gap-8 px-6 pt-12 pb-4 shrink-0">
            <div className="h-5 w-20 rounded bg-slate-700 animate-pulse" />
            <div className="h-4 w-14 rounded bg-slate-700/50 animate-pulse" />
          </header>
          {/* Skeleton icon grid */}
          <div className="grid grid-cols-3 lg:grid-cols-4 xl:grid-cols-6 gap-6 p-6 content-start">
            {Array.from({ length: 12 }).map((_, i) => (
              <div key={i} className="flex flex-col items-center gap-2">
                <div className="w-20 h-20 rounded-2xl bg-slate-800 animate-pulse" />
                <div className="h-3 w-14 rounded bg-slate-700 animate-pulse" />
              </div>
            ))}
          </div>
        </div>
      ) : (
        <HomescreenLayout agents={agents} apps={apps} />
      )}
      <BottomNav />
    </>
  )
}
