import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { useReducedMotion } from 'framer-motion'
import AppIcon from '@/components/AppIcon'
import type { App } from '@/lib/api'

// Mock framer-motion so motion.a renders as a plain <a> and hooks are controllable
jest.mock('framer-motion', () => {
  const React = require('react')
  return {
    motion: {
      a: React.forwardRef(
        (
          { children, whileHover, transition, ...props }: React.ComponentPropsWithoutRef<'a'> & { whileHover?: unknown; transition?: unknown },
          ref: React.Ref<HTMLAnchorElement>
        ) => (
          <a ref={ref} {...props}>
            {children}
          </a>
        )
      ),
    },
    useReducedMotion: jest.fn().mockReturnValue(false),
  }
})

function makeApp(overrides: Partial<App> = {}): App {
  return {
    id: 'test-id',
    name: 'test-app',
    slug: 'test-app',
    upstreamURL: 'http://localhost:3000',
    displayName: 'Test App',
    description: 'A test app',
    icon: '🤖',
    tags: [],
    enabled: true,
    health: 'healthy',
    createdAt: '2024-01-01T00:00:00Z',
    updatedAt: '2024-01-01T00:00:00Z',
    ...overrides,
  }
}

describe('AppIcon', () => {
  it('renders with emoji icon', () => {
    const { container } = render(<AppIcon app={makeApp({ icon: '🤖' })} />)
    // Emoji rendered in a <span>; no <img> tag present
    expect(container.querySelector('img')).toBeNull()
    expect(screen.getByText('🤖')).toBeInTheDocument()
  })

  it('renders with image URL icon', () => {
    const { container } = render(<AppIcon app={makeApp({ icon: 'https://example.com/icon.png' })} />)
    // Icon URLs render an <img> element (alt="" → role=presentation in ARIA)
    const img = container.querySelector('img') as HTMLImageElement
    expect(img).not.toBeNull()
    expect(img.src).toBe('https://example.com/icon.png')
  })

  it('falls back to default emoji when image URL fails to load', () => {
    const { container } = render(<AppIcon app={makeApp({ icon: 'https://example.com/broken.png' })} />)
    const img = container.querySelector('img') as HTMLImageElement
    expect(img).not.toBeNull()
    fireEvent.error(img)
    expect(screen.getByText('📦')).toBeInTheDocument()
  })

  it('shows green health dot for healthy state', () => {
    const { container } = render(<AppIcon app={makeApp({ health: 'healthy' })} />)
    const dot = container.querySelector('.bg-green-400')
    expect(dot).toBeInTheDocument()
    expect(dot).toHaveClass('animate-pulse')
  })

  it('shows amber health dot for unreachable state', () => {
    const { container } = render(<AppIcon app={makeApp({ health: 'unreachable' })} />)
    const dot = container.querySelector('.bg-amber-400')
    expect(dot).toBeInTheDocument()
    expect(dot).toHaveClass('animate-pulse')
  })

  it('shows gray static health dot for unknown state', () => {
    const { container } = render(<AppIcon app={makeApp({ health: 'unknown' })} />)
    const dot = container.querySelector('.bg-gray-400')
    expect(dot).toBeInTheDocument()
    expect(dot).not.toHaveClass('animate-pulse')
  })

  it('truncates a long display name', () => {
    const longName = 'A Very Long Display Name That Should Get Truncated'
    const { container } = render(<AppIcon app={makeApp({ displayName: longName })} />)
    const nameEl = container.querySelector('.truncate')
    expect(nameEl).toBeInTheDocument()
    expect(nameEl).toHaveTextContent(longName)
    expect(nameEl).toHaveClass('truncate')
  })

  it('renders aria-label with display name and health', () => {
    render(<AppIcon app={makeApp({ displayName: 'My App', health: 'healthy' })} />)
    expect(screen.getByRole('link', { name: /My App, healthy status/i })).toBeInTheDocument()
  })

  it('opens error dialog on click when unreachable', async () => {
    render(<AppIcon app={makeApp({ health: 'unreachable', displayName: 'My App' })} />)
    const link = screen.getByRole('link', { name: /My App, unreachable status/i })
    fireEvent.click(link)
    await waitFor(() => {
      expect(screen.getByText('App Unreachable')).toBeInTheDocument()
    })
  })

  it('does not open dialog on click when healthy', () => {
    render(<AppIcon app={makeApp({ health: 'healthy', displayName: 'My App' })} />)
    const link = screen.getByRole('link', { name: /My App, healthy status/i })
    fireEvent.click(link)
    expect(screen.queryByText('App Unreachable')).not.toBeInTheDocument()
  })

  it('renders correctly and remains accessible when prefers-reduced-motion is enabled', () => {
    ;(useReducedMotion as jest.Mock).mockReturnValueOnce(true)
    render(<AppIcon app={makeApp({ displayName: 'My App', health: 'healthy' })} />)
    // Link must still be present and labelled — no animation props should cause a crash
    expect(screen.getByRole('link', { name: /My App, healthy status/i })).toBeInTheDocument()
  })
})
