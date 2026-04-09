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
  onOpenProxyApp?: (app: App) => void
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
 * - `healthy` + `direct`: clicking opens the upstream URL in a new tab.
 * - `healthy` + `proxy`: clicking calls `onOpenProxyApp` to open the app in a
 *   full-screen iFrame served through the NGINX proxy at `/apps/<slug>/`.
 * - `unreachable`: clicking opens an error dialog regardless of access type.
 * - Hover lift animation is suppressed when `prefers-reduced-motion` is set.
 * - Image icons fall back to the 📦 emoji on load error.
 */
export default function AppIcon({ app, onOpenProxyApp }: AppIconProps) {
  const [errorDialogOpen, setErrorDialogOpen] = useState(false)
  const [imgError, setImgError] = useState(false)
  const prefersReducedMotion = useReducedMotion()

  const handleClick = (e: React.MouseEvent<HTMLAnchorElement>) => {
    if (app.health === 'unreachable') {
      e.preventDefault()
      setErrorDialogOpen(true)
      return
    }
    if (app.health === 'healthy' && app.accessType === 'proxy') {
      e.preventDefault()
      onOpenProxyApp?.(app)
      return
    }
    if (app.health !== 'healthy') {
      e.preventDefault()
      // unknown: prevent navigation but do nothing else
    }
    // healthy + direct: let the default <a> href handle navigation
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLAnchorElement>) => {
    if (e.key !== 'Enter' && e.key !== ' ') return
    e.preventDefault()

    if (app.health === 'unreachable') {
      setErrorDialogOpen(true)
    } else if (app.health === 'healthy') {
      if (app.accessType === 'proxy') {
        onOpenProxyApp?.(app)
      } else {
        window.open(app.upstreamURL, '_blank', 'noopener,noreferrer')
      }
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
        href={app.health === 'healthy' && app.accessType === 'direct' ? app.upstreamURL : '#'}
        target={app.health === 'healthy' && app.accessType === 'direct' ? '_blank' : undefined}
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
