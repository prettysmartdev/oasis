'use client'

import { useEffect, useState } from 'react'

function getGradientClasses(hour: number): string {
  if (hour >= 5 && hour < 9) {
    // Sunrise
    return 'from-orange-400 to-red-500'
  } else if (hour >= 9 && hour < 17) {
    // Midday
    return 'from-sky-400 to-white'
  } else if (hour >= 17 && hour < 21) {
    // Sunset
    return 'from-orange-500 to-red-800'
  } else {
    // Night
    return 'from-slate-950 to-slate-700'
  }
}

/**
 * Fixed full-viewport background gradient that reflects the time of day.
 *
 * The gradient is evaluated client-side only (`'use client'` + mounted guard)
 * to avoid hydration mismatches in the static export — the server has no clock.
 * It re-evaluates every 10 minutes so long-lived sessions transition naturally.
 *
 * Time ranges:
 *   - 05–08  sunrise  (orange → red)
 *   - 09–16  midday   (sky blue → white)
 *   - 17–20  sunset   (orange → deep red)
 *   - 21–04  night    (near-black → cool silver)
 */
export default function TimeOfDayBackground() {
  const [mounted, setMounted] = useState(false)
  const [gradientClasses, setGradientClasses] = useState('from-slate-950 to-slate-700')

  useEffect(() => {
    setMounted(true)
    const update = () => {
      const hour = new Date().getHours()
      setGradientClasses(getGradientClasses(hour))
    }
    update()

    const interval = setInterval(update, 10 * 60 * 1000)
    return () => clearInterval(interval)
  }, [])

  if (!mounted) {
    return (
      <div
        aria-hidden="true"
        className="fixed inset-0 -z-10 bg-slate-950"
      />
    )
  }

  return (
    <div
      aria-hidden="true"
      className={[
        'fixed inset-0 -z-10 bg-gradient-to-b',
        gradientClasses,
        'oasis-bg-animate',
      ].join(' ')}
    />
  )
}
