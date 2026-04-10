import React from 'react'
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react'
import ChatOverlay from '@/components/ChatOverlay'
import { getChatHistory, sendChatMessage } from '@/lib/api'

jest.mock('@/lib/api', () => ({
  getChatHistory: jest.fn(),
  sendChatMessage: jest.fn(),
  ApiError: class ApiError extends Error {
    constructor(
      public status: number,
      message: string
    ) {
      super(message)
    }
  },
}))

const mockGetChatHistory = getChatHistory as jest.Mock
const mockSendChatMessage = sendChatMessage as jest.Mock

describe('ChatOverlay', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    // Default: empty history
    mockGetChatHistory.mockResolvedValue({ items: [], total: 0 })
  })

  it('renders history returned by getChatHistory', async () => {
    mockGetChatHistory.mockResolvedValue({
      items: [
        {
          id: '1',
          role: 'user',
          content: 'Hello',
          createdAt: '2024-01-01T00:00:00Z',
        },
      ],
      total: 1,
    })

    render(<ChatOverlay onClose={jest.fn()} />)

    await waitFor(() => {
      expect(screen.getByText('Hello')).toBeInTheDocument()
    })
  })

  it('appends user bubble optimistically on send', async () => {
    mockGetChatHistory.mockResolvedValue({ items: [], total: 0 })
    // Never-resolving promise to keep the send in-flight
    mockSendChatMessage.mockReturnValue(new Promise(() => {}))

    render(<ChatOverlay onClose={jest.fn()} />)

    // Wait for history fetch to complete
    await act(async () => {})

    const input = screen.getByRole('textbox', { name: /Chat message/i })
    fireEvent.change(input, { target: { value: 'Test message' } })
    fireEvent.click(screen.getByRole('button', { name: /Send message/i }))

    // Optimistic bubble should appear before the promise resolves
    expect(screen.getByText('Test message')).toBeInTheDocument()
  })

  it('appends assistant bubble after sendChatMessage resolves', async () => {
    mockGetChatHistory.mockResolvedValue({ items: [], total: 0 })
    mockSendChatMessage.mockResolvedValue({
      userMessage: {
        id: 'u-1',
        role: 'user',
        content: 'Hello there',
        createdAt: '2024-01-01T00:00:00Z',
      },
      assistantMessage: {
        id: 'a-1',
        role: 'assistant',
        content: 'Hi! How can I help?',
        createdAt: '2024-01-01T00:00:01Z',
      },
    })

    render(<ChatOverlay onClose={jest.fn()} />)
    await act(async () => {})

    const input = screen.getByRole('textbox', { name: /Chat message/i })
    fireEvent.change(input, { target: { value: 'Hello there' } })
    fireEvent.click(screen.getByRole('button', { name: /Send message/i }))

    await waitFor(() => {
      expect(screen.getByText('Hi! How can I help?')).toBeInTheDocument()
    })
    expect(screen.getByText('Hello there')).toBeInTheDocument()
  })

  it('removes optimistic bubble and shows error toast on sendChatMessage reject', async () => {
    mockGetChatHistory.mockResolvedValue({ items: [], total: 0 })
    mockSendChatMessage.mockRejectedValue(new Error('server error'))

    render(<ChatOverlay onClose={jest.fn()} />)
    await act(async () => {})

    const input = screen.getByRole('textbox', { name: /Chat message/i })
    fireEvent.change(input, { target: { value: 'failing message' } })
    fireEvent.click(screen.getByRole('button', { name: /Send message/i }))

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument()
    })

    // Optimistic bubble should be gone
    expect(screen.queryByText('failing message')).not.toBeInTheDocument()

    // Error toast should show error text
    expect(screen.getByRole('alert')).toHaveTextContent('server error')

    // Input should be restored with the original message text
    expect(screen.getByRole('textbox', { name: /Chat message/i })).toHaveValue(
      'failing message'
    )
  })

  it('scrolls to bottom after new message', async () => {
    mockGetChatHistory.mockResolvedValue({
      items: [
        {
          id: '1',
          role: 'user',
          content: 'A message',
          createdAt: '2024-01-01T00:00:00Z',
        },
      ],
      total: 1,
    })

    render(<ChatOverlay onClose={jest.fn()} />)

    await waitFor(() => {
      expect(screen.getByText('A message')).toBeInTheDocument()
    })

    expect(Element.prototype.scrollTo).toHaveBeenCalled()
  })

  it('X button calls onClose', async () => {
    const mockOnClose = jest.fn()

    render(<ChatOverlay onClose={mockOnClose} />)
    await act(async () => {})

    fireEvent.click(screen.getByRole('button', { name: /Close chat/i }))
    expect(mockOnClose).toHaveBeenCalledTimes(1)
  })

  it('entry animation skipped when prefers-reduced-motion', async () => {
    const originalMatchMedia = window.matchMedia

    const mockMatchMedia = jest.fn().mockReturnValue({
      matches: true,
      media: '',
      onchange: null,
      addListener: jest.fn(),
      removeListener: jest.fn(),
      addEventListener: jest.fn(),
      removeEventListener: jest.fn(),
      dispatchEvent: jest.fn(),
    })
    Object.defineProperty(window, 'matchMedia', { writable: true, value: mockMatchMedia })

    render(<ChatOverlay onClose={jest.fn()} />)
    await act(async () => {})

    const overlay = screen.getByTestId('chat-overlay')
    // When reduced motion is set, transition should be 'none'
    expect(overlay.style.transition).toBe('none')

    // Restore original
    Object.defineProperty(window, 'matchMedia', { writable: true, value: originalMatchMedia })
  })
})
