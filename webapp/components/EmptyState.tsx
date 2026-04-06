interface EmptyStateProps {
  page: 'agents' | 'apps'
}

function AgentsSVG() {
  return (
    <svg
      width="160"
      height="120"
      viewBox="0 0 160 120"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden="true"
    >
      {/* Desert sky */}
      <rect width="160" height="120" rx="12" fill="#1e293b" />
      {/* Sand dunes */}
      <ellipse cx="80" cy="110" rx="90" ry="30" fill="#92400e" opacity="0.6" />
      <ellipse cx="30" cy="105" rx="50" ry="20" fill="#b45309" opacity="0.5" />
      <ellipse cx="130" cy="108" rx="45" ry="18" fill="#78350f" opacity="0.5" />
      {/* Palm tree trunk */}
      <rect x="76" y="55" width="8" height="40" rx="4" fill="#78350f" />
      {/* Palm tree leaves */}
      <ellipse cx="80" cy="50" rx="20" ry="8" fill="#166534" transform="rotate(-20 80 50)" />
      <ellipse cx="80" cy="50" rx="20" ry="8" fill="#166534" transform="rotate(20 80 50)" />
      <ellipse cx="80" cy="50" rx="20" ry="8" fill="#15803d" transform="rotate(60 80 50)" />
      <ellipse cx="80" cy="50" rx="20" ry="8" fill="#15803d" transform="rotate(-60 80 50)" />
      <ellipse cx="80" cy="45" rx="14" ry="6" fill="#16a34a" />
      {/* Stars */}
      <circle cx="20" cy="20" r="1.5" fill="white" opacity="0.8" />
      <circle cx="50" cy="10" r="1" fill="white" opacity="0.6" />
      <circle cx="100" cy="15" r="1.5" fill="white" opacity="0.9" />
      <circle cx="140" cy="25" r="1" fill="white" opacity="0.7" />
      <circle cx="35" cy="35" r="1" fill="white" opacity="0.5" />
      <circle cx="120" cy="30" r="1.5" fill="white" opacity="0.8" />
    </svg>
  )
}

function AppsSVG() {
  return (
    <svg
      width="160"
      height="120"
      viewBox="0 0 160 120"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden="true"
    >
      {/* Sky */}
      <rect width="160" height="120" rx="12" fill="#0c4a6e" />
      {/* Lake */}
      <ellipse cx="80" cy="95" rx="70" ry="20" fill="#0369a1" opacity="0.7" />
      {/* Lake reflection shimmer */}
      <ellipse cx="80" cy="95" rx="50" ry="10" fill="#38bdf8" opacity="0.2" />
      {/* Mountains */}
      <polygon points="0,90 40,40 80,90" fill="#1e3a5f" />
      <polygon points="50,90 90,35 130,90" fill="#1e4976" />
      <polygon points="100,90 140,50 160,90" fill="#1e3a5f" />
      {/* Snow caps */}
      <polygon points="40,40 32,58 48,58" fill="white" opacity="0.9" />
      <polygon points="90,35 82,55 98,55" fill="white" opacity="0.9" />
      {/* Cabin */}
      <rect x="64" y="72" width="32" height="20" fill="#7c3aed" />
      <polygon points="60,72 80,58 100,72" fill="#6d28d9" />
      {/* Cabin door */}
      <rect x="75" y="82" width="10" height="10" rx="1" fill="#4c1d95" />
      {/* Cabin window */}
      <rect x="66" y="75" width="8" height="8" rx="1" fill="#fde68a" opacity="0.8" />
      {/* Stars */}
      <circle cx="15" cy="15" r="1.5" fill="white" opacity="0.8" />
      <circle cx="45" cy="8" r="1" fill="white" opacity="0.6" />
      <circle cx="110" cy="12" r="1.5" fill="white" opacity="0.9" />
      <circle cx="145" cy="20" r="1" fill="white" opacity="0.7" />
      <circle cx="130" cy="8" r="1" fill="white" opacity="0.5" />
    </svg>
  )
}

/**
 * Empty-state screen shown on the Agents or Apps page when no items are
 * registered. Includes an inline SVG illustration, instructional copy, and a
 * code block showing the `oasis app add` command.
 */
export default function EmptyState({ page }: EmptyStateProps) {
  const isAgents = page === 'agents'

  return (
    <div className="flex flex-col items-center justify-center flex-1 px-8 py-16 text-center gap-6">
      <div className="opacity-90">
        {isAgents ? <AgentsSVG /> : <AppsSVG />}
      </div>

      <div className="space-y-2">
        <h2 className="text-2xl font-bold text-white">Your OaSis awaits</h2>
        <p className="text-slate-400 text-sm">
          Add your first {isAgents ? 'agent' : 'app'} from the terminal
        </p>
      </div>

      <pre className="bg-slate-900/80 border border-slate-700 rounded-lg px-6 py-3 font-mono text-sm text-primary">
        oasis app add
      </pre>
    </div>
  )
}
