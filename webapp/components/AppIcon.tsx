'use client'

import { useState } from 'react'
import { motion, useReducedMotion } from 'framer-motion'
import { type App } from '@/lib/api'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog'

interface AppIconProps {
  app: App
}

function isUrl(icon: string): boolean {
  return icon.startsWith('http://') || icon.startsWith('https://')
}

function HealthDot({ health }: { health: App['health'] }) {
  if (health === 'healthy') {
    return (
      <span
        className="absolute -bottom-1 -right-1 h-3 w-3 rounded-full bg-green-400 animate-pulse ring-2 ring-slate-800"
        aria-hidden="true"
      />
    )
  }
  if (health === 'unreachable') {
    return (
      <span
        className="absolute -bottom-1 -right-1 h-3 w-3 rounded-full bg-amber-400 animate-pulse ring-2 ring-slate-800"
        aria-hidden="true"
      />
    )
  }
  return (
    <span
      className="absolute -bottom-1 -right-1 h-3 w-3 rounded-full bg-gray-400 ring-2 ring-slate-800"
      aria-hidden="true"
    />
  )
}

/**
 * Renders a single app or agent as an iOS-style rounded-rect tile.
 *
 * - Clicking/tapping a `healthy` app opens the upstream URL in a new tab.
 * - Clicking an `unreachable` app opens an error dialog instead of navigating.
 * - Hover lift animation is suppressed when `prefers-reduced-motion` is set.
 * - Image icons fall back to the 📦 emoji on load error.
 */
export default function AppIcon({ app }: AppIconProps) {
  const [errorDialogOpen, setErrorDialogOpen] = useState(false)
  const [imgError, setImgError] = useState(false)
  const prefersReducedMotion = useReducedMotion()

  const handleClick = (e: React.MouseEvent<HTMLAnchorElement>) => {
    if (app.health === 'healthy') {
      // Let the default <a> href handle navigation for healthy apps
      return
    }
    e.preventDefault()
    if (app.health === 'unreachable') {
      setErrorDialogOpen(true)
    }
    // unknown: prevent navigation (href="#") but do nothing else
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLAnchorElement>) => {
    if (e.key !== 'Enter' && e.key !== ' ') return
    e.preventDefault()

    if (app.health === 'healthy') {
      window.open(app.upstreamURL, '_blank', 'noopener,noreferrer')
    } else if (app.health === 'unreachable') {
      setErrorDialogOpen(true)
    }
  }

  const motionProps = prefersReducedMotion
    ? {}
    : {
        whileHover: { y: -2 },
        transition: { type: 'spring' as const, stiffness: 400, damping: 20 },
      }

  return (
    <>
      <motion.a
        href={app.health === 'healthy' ? app.upstreamURL : '#'}
        target={app.health === 'healthy' ? '_blank' : undefined}
        rel="noopener noreferrer"
        tabIndex={0}
        aria-label={`${app.displayName}, ${app.health} status`}
        onClick={handleClick}
        onKeyDown={handleKeyDown}
        className="flex flex-col items-center gap-2 cursor-pointer select-none group focus:outline-none min-w-[44px] min-h-[44px]"
        {...motionProps}
      >
        <div className="relative">
          <div className="w-20 h-20 rounded-2xl bg-slate-800 border border-slate-700 flex items-center justify-center overflow-hidden group-focus-visible:ring-2 group-focus-visible:ring-primary group-focus-visible:ring-offset-2 group-focus-visible:ring-offset-transparent">
            {isUrl(app.icon) && !imgError ? (
              <img
                src={app.icon}
                alt=""
                aria-hidden="true"
                className="w-12 h-12 object-contain"
                onError={() => setImgError(true)}
              />
            ) : (
              <span className="text-4xl" role="img" aria-hidden="true">
                {!isUrl(app.icon) ? (app.icon || '📦') : '📦'}
              </span>
            )}
          </div>
          <HealthDot health={app.health} />
        </div>

        <span className="text-xs text-white text-center w-20 truncate font-sans">
          {app.displayName}
        </span>
      </motion.a>

      <Dialog open={errorDialogOpen} onOpenChange={setErrorDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>App Unreachable</DialogTitle>
            <DialogDescription>
              <strong className="text-white">{app.displayName}</strong> is currently unreachable.
              Check that the upstream service at{' '}
              <code className="font-mono text-primary text-xs">{app.upstreamURL}</code>{' '}
              is running.
            </DialogDescription>
          </DialogHeader>
        </DialogContent>
      </Dialog>
    </>
  )
}
