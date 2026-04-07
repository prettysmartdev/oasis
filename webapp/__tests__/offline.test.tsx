import React from 'react'
import { render, screen } from '@testing-library/react'
import OfflinePage from '../app/offline/page'

describe('OfflinePage', () => {
  beforeEach(() => {
    global.fetch = jest.fn()
  })

  afterEach(() => {
    delete (global as { fetch?: unknown }).fetch
  })

  it('renders without crashing', () => {
    render(<OfflinePage />)
    expect(document.body).toBeTruthy()
  })

  it('displays the offline headline', () => {
    render(<OfflinePage />)
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent("You're offline")
  })

  it('displays the reconnect subline', () => {
    render(<OfflinePage />)
    expect(screen.getByText(/couldn't reach your tailnet/i)).toBeInTheDocument()
  })

  it('makes no network calls', () => {
    render(<OfflinePage />)
    expect(global.fetch).not.toHaveBeenCalled()
  })
})
