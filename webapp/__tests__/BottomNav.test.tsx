import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import BottomNav from '@/components/BottomNav'

// Mock the API module so fetchStatus doesn't make real HTTP calls
jest.mock('@/lib/api', () => ({
  fetchStatus: jest.fn().mockResolvedValue({
    tailscaleConnected: true,
    tailscaleIP: '100.64.0.1',
    tailscaleHostname: 'oasis',
    nginxStatus: 'running',
    registeredAppCount: 3,
    version: '0.1.0-test',
  }),
}))

describe('BottomNav', () => {
  it('renders the settings button with correct ARIA label', () => {
    render(<BottomNav />)
    expect(screen.getByRole('button', { name: /Open settings/i })).toBeInTheDocument()
  })

  it('renders the logo/chat button with correct ARIA label', () => {
    render(<BottomNav />)
    expect(screen.getByRole('button', { name: /Open chat/i })).toBeInTheDocument()
  })

  it('opens settings dialog when settings button is clicked', async () => {
    render(<BottomNav />)
    const settingsBtn = screen.getByRole('button', { name: /Open settings/i })
    fireEvent.click(settingsBtn)
    await waitFor(() => {
      expect(screen.getByText('OaSis Status')).toBeInTheDocument()
    })
  })

  it('settings dialog shows status data after loading', async () => {
    render(<BottomNav />)
    fireEvent.click(screen.getByRole('button', { name: /Open settings/i }))
    await waitFor(() => {
      expect(screen.getByText('0.1.0-test')).toBeInTheDocument()
    })
  })

  it('logo button toggles chat input visibility', () => {
    render(<BottomNav />)
    const logoBtn = screen.getByRole('button', { name: /Open chat/i })
    // Initially aria-expanded=false
    expect(logoBtn).toHaveAttribute('aria-expanded', 'false')
    fireEvent.click(logoBtn)
    expect(logoBtn).toHaveAttribute('aria-expanded', 'true')
    fireEvent.click(logoBtn)
    expect(logoBtn).toHaveAttribute('aria-expanded', 'false')
  })

  it('settings dialog shows fallback message when status is unavailable', async () => {
    const { fetchStatus } = require('@/lib/api') as { fetchStatus: jest.Mock }
    fetchStatus.mockRejectedValueOnce(new Error('network error'))

    render(<BottomNav />)
    fireEvent.click(screen.getByRole('button', { name: /Open settings/i }))
    await waitFor(() => {
      expect(screen.getByText('OaSis Status')).toBeInTheDocument()
    })
    // Status null → fallback message shown
    expect(screen.getByText(/Unable to reach the OaSis controller/i)).toBeInTheDocument()
  })
})
