import React from 'react'
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react'
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

  it('outermost nav has bottom-nav-safe class for iOS safe-area inset', () => {
    render(<BottomNav />)
    const nav = screen.getByRole('navigation', { name: /Bottom navigation/i })
    expect(nav).toHaveClass('bottom-nav-safe')
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

  it('home button is present in the DOM and visible when appOpen=true and chatOpen=true', async () => {
    render(<BottomNav appOpen={true} onCloseApp={jest.fn()} />)
    // Flush the fetchStatus promise so React updates don't escape act()
    await act(async () => {})
    // Open chat to trigger chatOpen=true
    fireEvent.click(screen.getByRole('button', { name: /Open chat/i }))
    const homeBtn = screen.getByRole('button', { name: /Return to home screen/i })
    expect(homeBtn).toBeInTheDocument()
    // The wrapper div should have opacity 1
    const wrapper = homeBtn.parentElement as HTMLElement
    expect(wrapper.style.opacity).toBe('1')
    expect(wrapper.style.width).toBe('48px')
  })

  it('home button wrapper is hidden (opacity 0) when appOpen=false even when chatOpen=true', async () => {
    render(<BottomNav appOpen={false} onCloseApp={jest.fn()} />)
    // Flush the fetchStatus promise so React updates don't escape act()
    await act(async () => {})
    // Open chat to trigger chatOpen=true
    fireEvent.click(screen.getByRole('button', { name: /Open chat/i }))
    const homeBtn = screen.getByRole('button', { name: /Return to home screen/i })
    const wrapper = homeBtn.parentElement as HTMLElement
    expect(wrapper.style.opacity).toBe('0')
    expect(wrapper.style.width).toBe('0px')
  })

  it('home button click calls onCloseApp', async () => {
    const mockOnCloseApp = jest.fn()
    render(<BottomNav appOpen={true} onCloseApp={mockOnCloseApp} />)
    // Flush the fetchStatus promise so React updates don't escape act()
    await act(async () => {})
    // Open chat so the home button becomes visible
    fireEvent.click(screen.getByRole('button', { name: /Open chat/i }))
    const homeBtn = screen.getByRole('button', { name: /Return to home screen/i })
    fireEvent.click(homeBtn)
    expect(mockOnCloseApp).toHaveBeenCalledTimes(1)
  })

  it('settings button is not rendered when appOpen=true', async () => {
    render(<BottomNav appOpen={true} />)
    await act(async () => {})
    expect(screen.queryByRole('button', { name: /Open settings/i })).not.toBeInTheDocument()
  })

  // Spec: the input must NOT auto-focus when the palm tree is tapped.
  it('text input does not auto-focus when palm tree button is tapped', async () => {
    render(<BottomNav />)
    await act(async () => {})
    const logoBtn = screen.getByRole('button', { name: /Open chat/i })
    fireEvent.click(logoBtn)
    // The chat bar slides open but the input should not steal focus.
    const input = screen.getByRole('textbox', { name: /Chat with OaSis/i })
    expect(document.activeElement).not.toBe(input)
  })

  it('calls onChatOpen when text input receives focus', async () => {
    const mockOnChatOpen = jest.fn()
    render(<BottomNav onChatOpen={mockOnChatOpen} />)
    await act(async () => {})
    // Open the chat bar first
    fireEvent.click(screen.getByRole('button', { name: /Open chat/i }))
    // Then focus the input
    const input = screen.getByRole('textbox', { name: /Chat with OaSis/i })
    fireEvent.focus(input)
    expect(mockOnChatOpen).toHaveBeenCalledTimes(1)
  })
})
