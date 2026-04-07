import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import HomescreenLayout from '@/components/HomescreenLayout'
import type { App, Agent } from '@/lib/api'

// Mock AppIcon to keep tests focused on layout behaviour
jest.mock('@/components/AppIcon', () => {
  const React = require('react')
  return function MockAppIcon({ app }: { app: App }) {
    return <div data-testid={`app-icon-${app.id}`}>{app.displayName}</div>
  }
})

// Mock AgentIcon to keep tests focused on layout behaviour
jest.mock('@/components/AgentIcon', () => {
  const React = require('react')
  return function MockAgentIcon({ agent }: { agent: Agent }) {
    return <div data-testid={`agent-icon-${agent.id}`}>{agent.name}</div>
  }
})

// Mock framer-motion used transitively
jest.mock('framer-motion', () => ({
  motion: {
    a: require('react').forwardRef(
      ({ children, ...props }: React.ComponentPropsWithoutRef<'a'>, ref: React.Ref<HTMLAnchorElement>) => (
        <a ref={ref} {...props}>{children}</a>
      )
    ),
    button: require('react').forwardRef(
      ({ children, ...props }: React.ComponentPropsWithoutRef<'button'>, ref: React.Ref<HTMLButtonElement>) => (
        <button ref={ref} {...props}>{children}</button>
      )
    ),
  },
  useReducedMotion: jest.fn().mockReturnValue(false),
}))

function makeApp(overrides: Partial<App> = {}): App {
  return {
    id: 'app-1',
    name: 'app-one',
    slug: 'app-one',
    upstreamURL: 'http://localhost:3001',
    displayName: 'App One',
    description: '',
    icon: '🔧',
    tags: [],
    enabled: true,
    health: 'healthy',
    createdAt: '2024-01-01T00:00:00Z',
    updatedAt: '2024-01-01T00:00:00Z',
    ...overrides,
  }
}

function makeAgent(overrides: Partial<Agent> = {}): Agent {
  return {
    id: 'agent-1',
    name: 'agent-one',
    slug: 'agent-one',
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

describe('HomescreenLayout', () => {
  it('renders AGENTS title', () => {
    render(<HomescreenLayout agents={[]} apps={[]} />)
    expect(screen.getByText('AGENTS')).toBeInTheDocument()
  })

  it('renders APPS title', () => {
    render(<HomescreenLayout agents={[]} apps={[]} />)
    expect(screen.getByText('APPS')).toBeInTheDocument()
  })

  it('contains two page sections', () => {
    render(<HomescreenLayout agents={[]} apps={[]} />)
    expect(screen.getByRole('region', { name: /Agents/i })).toBeInTheDocument()
    expect(screen.getByRole('region', { name: /Apps/i })).toBeInTheDocument()
  })

  it('shows empty state when no agents', () => {
    render(<HomescreenLayout agents={[]} apps={[]} />)
    // EmptyState renders "Your OaSis awaits" for both sections
    const headings = screen.getAllByText(/Your OaSis awaits/i)
    expect(headings.length).toBeGreaterThanOrEqual(1)
  })

  it('renders agent icons when agents are provided', () => {
    const agents = [makeAgent({ id: 'a1', name: 'Agent One' })]
    render(<HomescreenLayout agents={agents} apps={[]} />)
    expect(screen.getByTestId('agent-icon-a1')).toBeInTheDocument()
  })

  it('renders app icons when apps are provided', () => {
    const apps = [makeApp({ id: 'app1', displayName: 'App One' })]
    render(<HomescreenLayout agents={[]} apps={apps} />)
    expect(screen.getByTestId('app-icon-app1')).toBeInTheDocument()
  })

  it('handles ArrowRight key on scroll container without error', () => {
    render(<HomescreenLayout agents={[]} apps={[]} />)
    const region = screen.getByRole('region', { name: /App pages/i })
    // scrollTo is a no-op in jsdom but must not throw
    expect(() => {
      fireEvent.keyDown(region, { key: 'ArrowRight' })
    }).not.toThrow()
  })

  it('handles ArrowLeft key on scroll container without error', () => {
    render(<HomescreenLayout agents={[]} apps={[]} />)
    const region = screen.getByRole('region', { name: /App pages/i })
    expect(() => {
      fireEvent.keyDown(region, { key: 'ArrowLeft' })
    }).not.toThrow()
  })

  it('scroll container has snap classes', () => {
    const { container } = render(<HomescreenLayout agents={[]} apps={[]} />)
    const scrollEl = container.querySelector('.snap-x.snap-mandatory')
    expect(scrollEl).toBeInTheDocument()
  })
})
