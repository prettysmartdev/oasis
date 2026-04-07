import React from 'react'
import { render } from '@testing-library/react'
import { metadata, viewport } from '../app/layout'
import RootLayout from '../app/layout'

// The Next.js Metadata API processes `metadata` and `viewport` exports during
// server-side rendering — they are injected into <head> by the framework, not
// by the React component. In Jest/jsdom, the framework pipeline does not run,
// so document.head will not contain the generated <link>/<meta> tags no matter
// how RootLayout is rendered. Full DOM-level assertions require an e2e test
// (e.g. Playwright hitting the live tsnet URL). We cover two layers instead:
//   1. Export-object tests — verify the PWA metadata config is correct.
//   2. Component render test — verify RootLayout mounts without crashing.

describe('layout metadata', () => {
  it('links the web app manifest', () => {
    expect(metadata.manifest).toBe('/manifest.json')
  })

  it('declares apple-web-app-capable for iOS standalone mode', () => {
    expect((metadata.appleWebApp as { capable: boolean }).capable).toBe(true)
  })

  it('sets apple-web-app-status-bar-style to black-translucent', () => {
    expect((metadata.appleWebApp as { statusBarStyle: string }).statusBarStyle).toBe('black-translucent')
  })

  it('sets apple-web-app-title', () => {
    expect((metadata.appleWebApp as { title: string }).title).toBe('OaSis')
  })

  it('includes the apple-touch-icon link', () => {
    expect((metadata.icons as { apple: string }).apple).toBe('/apple-touch-icon.png')
  })
})

describe('layout viewport', () => {
  it('sets theme-color to the primary brand teal', () => {
    expect(viewport.themeColor).toBe('#2DD4BF')
  })

  it('sets viewport-fit to cover for iOS notch / dynamic island', () => {
    expect(viewport.viewportFit).toBe('cover')
  })

  it('sets device-width', () => {
    expect(viewport.width).toBe('device-width')
  })

  it('disables pinch-zoom (icon launcher — not a reading surface)', () => {
    expect(viewport.userScalable).toBe(false)
    expect(viewport.maximumScale).toBe(1)
  })
})

describe('RootLayout component', () => {
  it('renders children without crashing', () => {
    // Verify the component tree mounts. The <head> meta tags emitted by the
    // Next.js Metadata API are validated by the export-object tests above and
    // by the Lighthouse PWA audit in the manual test plan.
    const { getByText } = render(
      <RootLayout>
        <div>test child</div>
      </RootLayout>
    )
    expect(getByText('test child')).toBeTruthy()
  })
})
