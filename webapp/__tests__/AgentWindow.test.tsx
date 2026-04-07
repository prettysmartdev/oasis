import React from 'react'
import { render, screen, act, waitFor, fireEvent } from '@testing-library/react'
import { AgentWindow } from '@/components/AgentWindow'
import type { Agent } from '@/lib/api'

// Stub API calls — real fetch is not available in jsdom
jest.mock('@/lib/api', () => {
  const actual = jest.requireActual('@/lib/api')
  return {
    ...actual,
    triggerAgentRun: jest.fn(),
    fetchAgentRun: jest.fn(),
    fetchLatestAgentRun: jest.fn(),
  }
})

import { triggerAgentRun, fetchAgentRun, fetchLatestAgentRun } from '@/lib/api'

const mockTriggerAgentRun = triggerAgentRun as jest.Mock
const mockFetchAgentRun = fetchAgentRun as jest.Mock
const mockFetchLatestAgentRun = fetchLatestAgentRun as jest.Mock

function makeAgent(overrides: Partial<Agent> = {}): Agent {
  return {
    id: 'agent-1',
    name: 'Test Agent',
    slug: 'test-agent',
    description: 'A test agent',
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

function doneRun(output = '# Hello\nWorld') {
  return {
    id: 'run-1',
    agentId: 'agent-1',
    triggerSrc: 'tap',
    status: 'done' as const,
    output,
    startedAt: '2024-01-01T00:00:00Z',
    finishedAt: '2024-01-01T00:00:01Z',
  }
}

describe('AgentWindow', () => {
  const onClose = jest.fn()

  beforeEach(() => {
    jest.useFakeTimers()
    jest.clearAllMocks()
  })

  afterEach(() => {
    jest.runOnlyPendingTimers()
    jest.useRealTimers()
  })

  it('renders without crashing for a tap agent', async () => {
    mockTriggerAgentRun.mockResolvedValue({ runId: 'run-1' })
    mockFetchAgentRun.mockResolvedValue(doneRun())

    await act(async () => {
      render(<AgentWindow agent={makeAgent()} onClose={onClose} />)
    })

    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  it('shows the agent name in the header', async () => {
    mockTriggerAgentRun.mockResolvedValue({ runId: 'run-1' })
    mockFetchAgentRun.mockResolvedValue(doneRun())

    await act(async () => {
      render(<AgentWindow agent={makeAgent({ name: 'My Agent' })} onClose={onClose} />)
    })

    expect(screen.getByText('My Agent')).toBeInTheDocument()
  })

  it('renders the close button', async () => {
    mockTriggerAgentRun.mockResolvedValue({ runId: 'run-1' })
    mockFetchAgentRun.mockResolvedValue(doneRun())

    await act(async () => {
      render(<AgentWindow agent={makeAgent()} onClose={onClose} />)
    })

    expect(screen.getByRole('button', { name: /Close agent window/i })).toBeInTheDocument()
  })

  it('calls onClose when Escape is pressed', async () => {
    mockTriggerAgentRun.mockResolvedValue({ runId: 'run-1' })
    mockFetchAgentRun.mockResolvedValue(doneRun())

    await act(async () => {
      render(<AgentWindow agent={makeAgent()} onClose={onClose} />)
    })

    act(() => {
      fireEvent.keyDown(document, { key: 'Escape' })
    })

    // onClose is called via setTimeout(onClose, 200) for animation; advance timers
    act(() => {
      jest.advanceTimersByTime(300)
    })

    expect(onClose).toHaveBeenCalled()
  })

  it('fetches latest run for a schedule agent', async () => {
    mockFetchLatestAgentRun.mockResolvedValue(doneRun('Schedule output'))

    await act(async () => {
      render(<AgentWindow agent={makeAgent({ trigger: 'schedule' })} onClose={onClose} />)
    })

    await waitFor(() => {
      expect(mockFetchLatestAgentRun).toHaveBeenCalledWith('test-agent')
    })
  })

  it('shows empty state when schedule agent has no runs', async () => {
    const { ApiError } = jest.requireActual('@/lib/api')
    mockFetchLatestAgentRun.mockRejectedValue(new ApiError(404, 'not found'))

    await act(async () => {
      render(<AgentWindow agent={makeAgent({ trigger: 'schedule' })} onClose={onClose} />)
    })

    await waitFor(() => {
      expect(screen.getByLabelText('No runs yet')).toBeInTheDocument()
    })
  })

  it('shows plaintext output in a pre element', async () => {
    mockTriggerAgentRun.mockResolvedValue({ runId: 'run-1' })
    mockFetchAgentRun.mockResolvedValue(doneRun('plain text output'))

    await act(async () => {
      render(
        <AgentWindow
          agent={makeAgent({ outputFmt: 'plaintext' })}
          onClose={onClose}
        />
      )
    })

    await waitFor(() => {
      expect(screen.getByText('plain text output')).toBeInTheDocument()
    })

    const { container } = render(
      <AgentWindow
        agent={makeAgent({ outputFmt: 'plaintext' })}
        onClose={onClose}
      />
    )
    await act(async () => {})
    expect(container.querySelector('pre')).toBeDefined()
  })

  it('shows an iframe for html output', async () => {
    mockTriggerAgentRun.mockResolvedValue({ runId: 'run-1' })
    mockFetchAgentRun.mockResolvedValue(doneRun('<p>hello</p>'))

    const { container } = await act(async () =>
      render(
        <AgentWindow
          agent={makeAgent({ outputFmt: 'html' })}
          onClose={onClose}
        />
      )
    )

    await waitFor(() => {
      const iframe = container.querySelector('iframe')
      expect(iframe).toBeInTheDocument()
      expect(iframe?.getAttribute('sandbox')).toBe('allow-scripts')
    })
  })

  it('shows error state when run status is error', async () => {
    mockTriggerAgentRun.mockResolvedValue({ runId: 'run-1' })
    mockFetchAgentRun.mockResolvedValue({
      ...doneRun(),
      status: 'error',
      output: 'something went wrong',
    })

    await act(async () => {
      render(<AgentWindow agent={makeAgent()} onClose={onClose} />)
    })

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument()
      expect(screen.getByText(/Agent run failed/i)).toBeInTheDocument()
    })
  })
})
