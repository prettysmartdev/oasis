import React from 'react'
import { render, screen, waitFor, act } from '@testing-library/react'
import HomePage from '../app/page'
import type { Agent, App } from '@/lib/api'

// Mock the API module to control what fetchApps and fetchAgents return
jest.mock('@/lib/api', () => {
  const actual = jest.requireActual('@/lib/api')
  return {
    ...actual,
    fetchApps: jest.fn(),
    fetchAgents: jest.fn(),
    fetchStatus: jest.fn(),
  }
})

import { fetchApps, fetchAgents, fetchStatus } from '@/lib/api'

const mockFetchApps = fetchApps as jest.Mock
const mockFetchAgents = fetchAgents as jest.Mock
const mockFetchStatus = fetchStatus as jest.Mock

// Suppress framer-motion motion.button usage in AgentIcon
jest.mock('framer-motion', () => {
  const React = require('react')
  return {
    motion: {
      a: React.forwardRef(
        ({ children, whileHover, transition, ...props }: React.ComponentPropsWithoutRef<'a'> & { whileHover?: unknown; transition?: unknown }, ref: React.Ref<HTMLAnchorElement>) => (
          <a ref={ref} {...props}>{children}</a>
        )
      ),
      button: React.forwardRef(
        ({ children, whileHover, transition, ...props }: React.ComponentPropsWithoutRef<'button'> & { whileHover?: unknown; transition?: unknown }, ref: React.Ref<HTMLButtonElement>) => (
          <button ref={ref} {...props}>{children}</button>
        )
      ),
    },
    useReducedMotion: jest.fn().mockReturnValue(false),
  }
})

const emptyApps = { items: [] as App[], total: 0 }
const emptyAgents = { items: [] as Agent[], total: 0 }

function makeAgent(overrides: Partial<Agent> = {}): Agent {
  return {
    id: 'agent-1',
    name: 'Test Agent',
    slug: 'test-agent',
    description: '',
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

describe('HomePage', () => {
  beforeEach(() => {
    mockFetchApps.mockResolvedValue(emptyApps)
    mockFetchAgents.mockResolvedValue(emptyAgents)
    mockFetchStatus.mockResolvedValue({
      tailscaleConnected: true,
      tailscaleIP: '100.64.0.1',
      tailscaleHostname: 'oasis',
      nginxStatus: 'running',
      registeredAppCount: 0,
      version: '0.1.0',
    })
  })

  afterEach(() => {
    jest.resetAllMocks()
  })

  it('renders without crashing', async () => {
    await act(async () => {
      render(<HomePage />)
    })
    expect(document.body).toBeTruthy()
  })

  it('renders without crashing when the API returns empty lists', async () => {
    await act(async () => {
      render(<HomePage />)
    })
    await waitFor(() => {
      expect(screen.getByText('AGENTS')).toBeInTheDocument()
      expect(screen.getByText('APPS')).toBeInTheDocument()
    })
  })

  it('shows an error banner when the apps API returns an error', async () => {
    const { ApiError } = jest.requireActual('@/lib/api')
    mockFetchApps.mockRejectedValue(new ApiError(503, 'controller offline'))

    await act(async () => {
      render(<HomePage />)
    })

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument()
      expect(screen.getByRole('alert')).toHaveTextContent('controller offline')
    })
  })

  it('shows a generic error banner on network failure', async () => {
    mockFetchApps.mockRejectedValue(new TypeError('Failed to fetch'))

    await act(async () => {
      render(<HomePage />)
    })

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument()
      expect(screen.getByRole('alert')).toHaveTextContent('Cannot reach OaSis controller')
    })
  })

  it('renders agent icons when agents API returns items', async () => {
    const agentItem = makeAgent({ id: 'a1', name: 'Agent One' })
    mockFetchAgents.mockResolvedValue({ items: [agentItem], total: 1 })

    await act(async () => {
      render(<HomePage />)
    })

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Open agent: Agent One/i })).toBeInTheDocument()
    })
  })

  it('degrades gracefully when the agents endpoint returns 404', async () => {
    const { ApiError } = jest.requireActual('@/lib/api')
    mockFetchAgents.mockRejectedValue(new ApiError(404, 'not found'))

    await act(async () => {
      render(<HomePage />)
    })

    // Should still render without an error banner — agents 404 is treated as empty
    await waitFor(() => {
      expect(screen.getByText('AGENTS')).toBeInTheDocument()
    })
    expect(screen.queryByRole('alert')).toBeNull()
  })
})
