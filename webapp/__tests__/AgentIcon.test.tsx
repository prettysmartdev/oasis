import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import AgentIcon from '@/components/AgentIcon'
import type { Agent } from '@/lib/api'

// Suppress framer-motion animation in tests
jest.mock('framer-motion', () => {
  const React = require('react')
  return {
    motion: {
      button: React.forwardRef(
        ({ children, whileHover, transition, ...props }: React.ComponentPropsWithoutRef<'button'> & { whileHover?: unknown; transition?: unknown }, ref: React.Ref<HTMLButtonElement>) => (
          <button ref={ref} {...props}>{children}</button>
        )
      ),
    },
    useReducedMotion: jest.fn().mockReturnValue(false),
  }
})

// Stub AgentWindow so we don't need to mock its dependencies
jest.mock('@/components/AgentWindow', () => ({
  AgentWindow: function MockAgentWindow({ onClose }: { agent: unknown; onClose: () => void }) {
    return (
      <div data-testid="agent-window">
        <button onClick={onClose}>Close</button>
      </div>
    )
  },
}))

function makeAgent(overrides: Partial<Agent> = {}): Agent {
  return {
    id: 'agent-1',
    name: 'Test Agent',
    slug: 'test-agent',
    description: 'A test agent',
    icon: '🤖',
    prompt: 'Do something',
    trigger: 'tap',
    schedule: '',
    outputFmt: 'markdown',
    enabled: true,
    createdAt: '2024-01-01T00:00:00Z',
    updatedAt: '2024-01-01T00:00:00Z',
    ...overrides,
  }
}

describe('AgentIcon', () => {
  it('renders without crashing', () => {
    render(<AgentIcon agent={makeAgent()} />)
    expect(document.body).toBeTruthy()
  })

  it('renders the agent name as label', () => {
    render(<AgentIcon agent={makeAgent({ name: 'My Agent' })} />)
    expect(screen.getByText('My Agent')).toBeInTheDocument()
  })

  it('renders the button with correct aria-label', () => {
    render(<AgentIcon agent={makeAgent({ name: 'My Agent' })} />)
    expect(screen.getByRole('button', { name: /Open agent: My Agent/i })).toBeInTheDocument()
  })

  it('shows an emoji icon for non-URL icons', () => {
    render(<AgentIcon agent={makeAgent({ icon: '🚀' })} />)
    expect(screen.getByText('🚀')).toBeInTheDocument()
  })

  it('renders an img element for URL icons', () => {
    const { container } = render(
      <AgentIcon agent={makeAgent({ icon: 'https://example.com/icon.png' })} />
    )
    const img = container.querySelector('img')
    expect(img).toBeInTheDocument()
    expect(img?.getAttribute('src')).toBe('https://example.com/icon.png')
  })

  it('opens AgentWindow when clicked', () => {
    render(<AgentIcon agent={makeAgent()} />)
    const btn = screen.getByRole('button', { name: /Open agent/i })
    fireEvent.click(btn)
    expect(screen.getByTestId('agent-window')).toBeInTheDocument()
  })

  it('closes AgentWindow when onClose is called', () => {
    render(<AgentIcon agent={makeAgent()} />)
    fireEvent.click(screen.getByRole('button', { name: /Open agent/i }))
    expect(screen.getByTestId('agent-window')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /Close/i }))
    expect(screen.queryByTestId('agent-window')).toBeNull()
  })

  it('renders a teal trigger dot for tap trigger', () => {
    const { container } = render(<AgentIcon agent={makeAgent({ trigger: 'tap' })} />)
    const dot = container.querySelector('.bg-teal-400')
    expect(dot).toBeInTheDocument()
  })

  it('renders an amber trigger dot for schedule trigger', () => {
    const { container } = render(<AgentIcon agent={makeAgent({ trigger: 'schedule' })} />)
    const dot = container.querySelector('.bg-amber-400')
    expect(dot).toBeInTheDocument()
  })

  it('renders a purple trigger dot for webhook trigger', () => {
    const { container } = render(<AgentIcon agent={makeAgent({ trigger: 'webhook' })} />)
    const dot = container.querySelector('.bg-purple-400')
    expect(dot).toBeInTheDocument()
  })
})
