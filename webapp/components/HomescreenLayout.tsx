'use client'

import { useEffect, useRef, useState, useCallback } from 'react'
import { type App } from '@/lib/api'
import AppIcon from '@/components/AppIcon'
import EmptyState from '@/components/EmptyState'

/**
 * Two-page horizontal scroll container for the OaSis dashboard.
 *
 * On mobile the two pages (Agents / Apps) are full-width and snap into place.
 * On desktop (≥ 1024 px) both columns are displayed side-by-side.
 *
 * The AGENTS / APPS header titles animate their size and opacity based on the
 * horizontal scroll position so the active page is visually prominent.
 * Left/right arrow keys switch between pages for keyboard users.
 */
interface HomescreenLayoutProps {
  agents: App[]
  apps: App[]
}

export default function HomescreenLayout({ agents, apps }: HomescreenLayoutProps) {
  const scrollRef = useRef<HTMLDivElement>(null)
  const [scrollProgress, setScrollProgress] = useState(0) // 0 = agents, 1 = apps

  const scrollToPage = useCallback((page: 0 | 1) => {
    const el = scrollRef.current
    if (!el) return
    el.scrollTo({ left: page * el.clientWidth, behavior: 'smooth' })
  }, [])

  const handleScroll = useCallback(() => {
    const el = scrollRef.current
    if (!el) return
    const progress = el.scrollLeft / (el.scrollWidth - el.clientWidth || 1)
    setScrollProgress(Math.min(1, Math.max(0, progress)))
  }, [])

  useEffect(() => {
    const el = scrollRef.current
    if (!el) return
    el.addEventListener('scroll', handleScroll, { passive: true })
    return () => el.removeEventListener('scroll', handleScroll)
  }, [handleScroll])

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === 'ArrowRight') {
        e.preventDefault()
        scrollToPage(1)
      } else if (e.key === 'ArrowLeft') {
        e.preventDefault()
        scrollToPage(0)
      }
    },
    [scrollToPage]
  )

  // Title opacity/size based on scroll progress
  const agentsActive = 1 - scrollProgress
  const appsActive = scrollProgress

  return (
    <div className="flex flex-col h-screen overflow-hidden">
      {/* Page title bar */}
      <header className="flex items-end gap-8 px-6 pt-12 pb-4 shrink-0">
        <button
          onClick={() => scrollToPage(0)}
          className="focus:outline-none focus-visible:ring-2 focus-visible:ring-primary rounded"
          aria-label="Go to Agents page"
        >
          <span
            className="font-bold text-white transition-all duration-300"
            style={{
              fontSize: `${1 + agentsActive * 0.25}rem`,
              opacity: 0.4 + agentsActive * 0.6,
            }}
          >
            AGENTS
          </span>
        </button>

        <button
          onClick={() => scrollToPage(1)}
          className="focus:outline-none focus-visible:ring-2 focus-visible:ring-primary rounded"
          aria-label="Go to Apps page"
        >
          <span
            className="font-bold text-white transition-all duration-300"
            style={{
              fontSize: `${1 + appsActive * 0.25}rem`,
              opacity: 0.4 + appsActive * 0.6,
            }}
          >
            APPS
          </span>
        </button>
      </header>

      {/* Horizontal scroll container */}
      <div
        ref={scrollRef}
        role="region"
        aria-label="App pages"
        tabIndex={0}
        onKeyDown={handleKeyDown}
        className="flex flex-1 overflow-x-auto overflow-y-hidden snap-x snap-mandatory scrollbar-hide focus:outline-none"
        style={{ scrollbarWidth: 'none' }}
      >
        {/* Agents page */}
        <section
          aria-label="Agents"
          className="snap-start shrink-0 w-full lg:w-1/2 flex flex-col overflow-y-auto"
        >
          {agents.length === 0 ? (
            <EmptyState page="agents" />
          ) : (
            <div className="grid grid-cols-3 lg:grid-cols-4 xl:grid-cols-6 gap-6 p-6 pb-32 content-start">
              {agents.map((agent) => (
                <AppIcon key={agent.id} app={agent} />
              ))}
            </div>
          )}
        </section>

        {/* Apps page */}
        <section
          aria-label="Apps"
          className="snap-start shrink-0 w-full lg:w-1/2 flex flex-col overflow-y-auto"
        >
          {apps.length === 0 ? (
            <EmptyState page="apps" />
          ) : (
            <div className="grid grid-cols-3 lg:grid-cols-4 xl:grid-cols-6 gap-6 p-6 pb-32 content-start">
              {apps.map((app) => (
                <AppIcon key={app.id} app={app} />
              ))}
            </div>
          )}
        </section>
      </div>
    </div>
  )
}
