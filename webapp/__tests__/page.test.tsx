import React from 'react'
import { render, screen, waitFor, act } from '@testing-library/react'
import HomePage from '../app/page'

// Suppress framer-motion motion.a usage in AppIcon
jest.mock('framer-motion', () => {
  const React = require('react')
  return {
    motion: {
      a: React.forwardRef(
        ({ children, whileHover, transition, ...props }: React.ComponentPropsWithoutRef<'a'> & { whileHover?: unknown; transition?: unknown }, ref: React.Ref<HTMLAnchorElement>) => (
          <a ref={ref} {...props}>{children}</a>
        )
      ),
    },
    useReducedMotion: jest.fn().mockReturnValue(false),
  }
})

// Default fetch mock: empty list, resolves immediately
function mockFetchEmpty() {
  global.fetch = jest.fn().mockResolvedValue({
    ok: true,
    json: async () => ({ items: [], total: 0 }),
  } as unknown as Response)
}

describe('HomePage', () => {
  beforeEach(() => {
    mockFetchEmpty()
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

  it('renders without crashing when the API returns an empty list', async () => {
    mockFetchEmpty()
    await act(async () => {
      render(<HomePage />)
    })
    // After data loads, layout renders with empty-state content
    await waitFor(() => {
      expect(screen.getByText('AGENTS')).toBeInTheDocument()
      expect(screen.getByText('APPS')).toBeInTheDocument()
    })
  })

  it('shows an error banner when the API returns a non-200 status', async () => {
    global.fetch = jest.fn().mockResolvedValue({
      ok: false,
      status: 503,
      statusText: 'Service Unavailable',
      json: async () => ({ error: 'controller offline' }),
    } as unknown as Response)

    await act(async () => {
      render(<HomePage />)
    })

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument()
      expect(screen.getByRole('alert')).toHaveTextContent('controller offline')
    })
  })

  it('shows a generic error banner on network failure', async () => {
    global.fetch = jest.fn().mockRejectedValue(new TypeError('Failed to fetch'))

    await act(async () => {
      render(<HomePage />)
    })

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument()
      expect(screen.getByRole('alert')).toHaveTextContent('Cannot reach OaSis controller')
    })
  })

  it('renders agent icons when API returns items tagged agent', async () => {
    global.fetch = jest.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        items: [
          {
            id: 'a1',
            name: 'agent-one',
            slug: 'agent-one',
            upstreamURL: 'http://localhost:8001',
            displayName: 'Agent One',
            description: '',
            icon: '🤖',
            tags: ['agent'],
            enabled: true,
            health: 'healthy',
            createdAt: '2024-01-01T00:00:00Z',
            updatedAt: '2024-01-01T00:00:00Z',
          },
        ],
        total: 1,
      }),
    } as unknown as Response)

    await act(async () => {
      render(<HomePage />)
    })

    await waitFor(() => {
      expect(screen.getByRole('link', { name: /Agent One, healthy status/i })).toBeInTheDocument()
    })
  })
})
