import React from 'react'
import { render, screen } from '@testing-library/react'
import { useReducedMotion } from 'framer-motion'
import { AppProxyView } from '@/components/AppProxyView'
import type { App } from '@/lib/api'

// Mock framer-motion: motion.div renders as a plain div with data attributes
// so we can inspect initial/animate props; useReducedMotion is controllable.
jest.mock('framer-motion', () => {
  const React = require('react')
  return {
    motion: {
      div: React.forwardRef(
        (
          { children, initial, animate, transition, ...props }: React.ComponentPropsWithoutRef<'div'> & {
            initial?: unknown
            animate?: unknown
            transition?: unknown
          },
          ref: React.Ref<HTMLDivElement>
        ) => (
          <div
            ref={ref}
            data-initial={initial !== undefined ? JSON.stringify(initial) : undefined}
            data-animate={animate !== undefined ? JSON.stringify(animate) : undefined}
            {...props}
          >
            {children}
          </div>
        )
      ),
    },
    useReducedMotion: jest.fn().mockReturnValue(false),
  }
})

function makeApp(overrides: Partial<App> = {}): App {
  return {
    id: 'proxy-test-id',
    name: 'proxy-app',
    slug: 'proxy-app',
    upstreamURL: 'http://localhost:4000',
    displayName: 'Proxy App',
    description: 'A proxy test app',
    icon: '🌐',
    tags: [],
    enabled: true,
    health: 'healthy',
    accessType: 'proxy',
    createdAt: '2024-01-01T00:00:00Z',
    updatedAt: '2024-01-01T00:00:00Z',
    ...overrides,
  }
}

describe('AppProxyView', () => {
  it('renders an iframe with src /apps/<slug>/ and title matching displayName', () => {
    const app = makeApp({ slug: 'my-proxy-app', displayName: 'My Proxy App' })
    const { container } = render(<AppProxyView app={app} />)
    const iframe = container.querySelector('iframe') as HTMLIFrameElement
    expect(iframe).not.toBeNull()
    // jsdom prepends the document origin, so we check with .includes
    expect(iframe.src).toContain('/apps/my-proxy-app/')
    expect(iframe.title).toBe('My Proxy App')
  })

  it('iframe occupies full screen (is present in the DOM)', () => {
    const app = makeApp()
    const { container } = render(<AppProxyView app={app} />)
    const iframe = container.querySelector('iframe')
    expect(iframe).toBeInTheDocument()
  })

  it('does not render iframe when app is null (conditional rendering by caller)', () => {
    function Wrapper({ app }: { app: App | null }) {
      return <>{app && <AppProxyView app={app} />}</>
    }
    const { container } = render(<Wrapper app={null} />)
    expect(container.querySelector('iframe')).toBeNull()
  })

  it('skips animation props when prefers-reduced-motion is true', () => {
    ;(useReducedMotion as jest.Mock).mockReturnValueOnce(true)
    const app = makeApp()
    const { container } = render(<AppProxyView app={app} />)
    // The motion.div wrapper is the first child of the container
    const wrapper = container.firstElementChild as HTMLElement
    // When prefersReducedMotion is true, initial and animate are undefined,
    // so data-initial and data-animate attributes should not be set on the element.
    expect(wrapper.getAttribute('data-initial')).toBeNull()
    expect(wrapper.getAttribute('data-animate')).toBeNull()
    // Iframe should still render
    expect(container.querySelector('iframe')).toBeInTheDocument()
  })
})
