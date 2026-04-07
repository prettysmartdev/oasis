'use client'

/**
 * AgentIcon — renders a single agent as an iOS-style rounded-rect tile.
 *
 * Clicking/tapping the icon opens the AgentWindow overlay.
 * For tap-triggered agents, opening triggers an immediate run.
 * For schedule/webhook agents, opening shows the most recent run output.
 *
 * Hover lift animation is suppressed when `prefers-reduced-motion` is set.
 * Image icons fall back to the 🤖 emoji on load error.
 */

import { useState } from 'react'
import { motion, useReducedMotion } from 'framer-motion'
import { type Agent } from '@/lib/api'
import { AgentWindow } from '@/components/AgentWindow'

interface AgentIconProps {
  agent: Agent
}

function isUrl(icon: string): boolean {
  return icon.startsWith('http://') || icon.startsWith('https://')
}

export default function AgentIcon({ agent }: AgentIconProps) {
  const [windowOpen, setWindowOpen] = useState(false)
  const [imgError, setImgError] = useState(false)
  const prefersReducedMotion = useReducedMotion()

  const motionProps = prefersReducedMotion
    ? {}
    : {
        whileHover: { y: -2 },
        transition: { type: 'spring' as const, stiffness: 400, damping: 20 },
      }

  return (
    <>
      <motion.button
        type="button"
        aria-label={`Open agent: ${agent.name}`}
        onClick={() => setWindowOpen(true)}
        className="flex flex-col items-center gap-2 cursor-pointer select-none group focus:outline-none min-w-[44px] min-h-[44px]"
        {...motionProps}
      >
        <div className="relative">
          <div className="w-20 h-20 rounded-2xl bg-slate-800 border border-slate-700 flex items-center justify-center overflow-hidden group-focus-visible:ring-2 group-focus-visible:ring-primary group-focus-visible:ring-offset-2 group-focus-visible:ring-offset-transparent">
            {isUrl(agent.icon) && !imgError ? (
              <img
                src={agent.icon}
                alt=""
                aria-hidden="true"
                className="w-12 h-12 object-contain"
                onError={() => setImgError(true)}
              />
            ) : (
              <span className="text-4xl" role="img" aria-hidden="true">
                {!isUrl(agent.icon) ? (agent.icon || '🤖') : '🤖'}
              </span>
            )}
          </div>
          {/* Trigger indicator dot */}
          <span
            aria-hidden="true"
            className={`absolute -bottom-1 -right-1 h-3 w-3 rounded-full ring-2 ring-slate-800 ${
              agent.trigger === 'tap'
                ? 'bg-teal-400'
                : agent.trigger === 'schedule'
                ? 'bg-amber-400'
                : 'bg-purple-400'
            } ${!agent.enabled ? 'opacity-40' : ''}`}
          />
        </div>
        <span className="text-xs text-white text-center w-20 truncate font-sans">
          {agent.name}
        </span>
      </motion.button>

      {windowOpen && (
        <AgentWindow agent={agent} onClose={() => setWindowOpen(false)} />
      )}
    </>
  )
}
