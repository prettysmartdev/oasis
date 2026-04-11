'use client'

import { motion, useReducedMotion } from 'framer-motion'
import { type App } from '@/lib/api'

interface AppProxyViewProps {
  app: App
}

/**
 * Full-screen overlay that renders a proxied app in an iFrame.
 *
 * Sits at z-30, beneath BottomNav (z-40) and above the homescreen grid.
 * No sandbox attribute — the app is user-registered and served from the same
 * origin via NGINX proxy; sandbox would break most web apps.
 *
 * Animates with a fade-in transition; skips animation when prefers-reduced-motion.
 */
export function AppProxyView({ app }: AppProxyViewProps) {
  const prefersReducedMotion = useReducedMotion()

  return (
    <motion.div
      className="fixed inset-0 z-30 flex flex-col notch-safe-top"
      initial={prefersReducedMotion ? undefined : { opacity: 0 }}
      animate={prefersReducedMotion ? undefined : { opacity: 1 }}
      transition={{ duration: 0.2 }}
    >
      {/* The safe-area strip above the iframe is transparent so the wallpaper
          gradient shows through behind the notch/Dynamic Island */}
      <iframe
        src={`/apps/${app.slug}/`}
        title={app.displayName}
        className="flex-1 w-full border-none min-h-0"
        allow="fullscreen"
      />
    </motion.div>
  )
}
