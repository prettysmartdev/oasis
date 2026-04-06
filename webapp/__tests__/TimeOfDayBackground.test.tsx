import React from 'react'
import { render, act } from '@testing-library/react'
import TimeOfDayBackground from '@/components/TimeOfDayBackground'

// Helper: mock window.matchMedia
function mockMatchMedia(matches: boolean) {
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: jest.fn().mockImplementation((query: string) => ({
      matches,
      media: query,
      onchange: null,
      addListener: jest.fn(),
      removeListener: jest.fn(),
      addEventListener: jest.fn(),
      removeEventListener: jest.fn(),
      dispatchEvent: jest.fn(),
    })),
  })
}

function mockHour(hour: number) {
  jest.spyOn(Date.prototype, 'getHours').mockReturnValue(hour)
}

afterEach(() => {
  jest.restoreAllMocks()
})

describe('TimeOfDayBackground', () => {
  it('renders a fixed background div', () => {
    const { container } = render(<TimeOfDayBackground />)
    const el = container.querySelector('[aria-hidden="true"]')
    expect(el).toBeInTheDocument()
  })

  it('applies sunrise gradient for hour 6', async () => {
    mockHour(6)
    let container!: HTMLElement
    await act(async () => {
      ;({ container } = render(<TimeOfDayBackground />))
    })
    const el = container.querySelector('[aria-hidden="true"]')
    expect(el?.className).toMatch(/from-orange-400/)
    expect(el?.className).toMatch(/to-red-500/)
  })

  it('applies midday gradient for hour 12', async () => {
    mockHour(12)
    let container!: HTMLElement
    await act(async () => {
      ;({ container } = render(<TimeOfDayBackground />))
    })
    const el = container.querySelector('[aria-hidden="true"]')
    expect(el?.className).toMatch(/from-sky-400/)
    expect(el?.className).toMatch(/to-white/)
  })

  it('applies sunset gradient for hour 18', async () => {
    mockHour(18)
    let container!: HTMLElement
    await act(async () => {
      ;({ container } = render(<TimeOfDayBackground />))
    })
    const el = container.querySelector('[aria-hidden="true"]')
    expect(el?.className).toMatch(/from-orange-500/)
    expect(el?.className).toMatch(/to-red-800/)
  })

  it('applies night gradient for hour 1', async () => {
    mockHour(1)
    let container!: HTMLElement
    await act(async () => {
      ;({ container } = render(<TimeOfDayBackground />))
    })
    const el = container.querySelector('[aria-hidden="true"]')
    expect(el?.className).toMatch(/from-slate-950/)
    expect(el?.className).toMatch(/to-slate-700/)
  })

  it('renders without crashing when prefers-reduced-motion is set', async () => {
    mockMatchMedia(true) // prefers-reduced-motion: reduce
    mockHour(12)
    let container!: HTMLElement
    await act(async () => {
      ;({ container } = render(<TimeOfDayBackground />))
    })
    // Component should still render; CSS handles animation suppression
    const el = container.querySelector('[aria-hidden="true"]')
    expect(el).toBeInTheDocument()
  })

  it('includes the animation class so CSS media query can gate it', async () => {
    mockMatchMedia(false) // prefers motion
    mockHour(12)
    let container!: HTMLElement
    await act(async () => {
      ;({ container } = render(<TimeOfDayBackground />))
    })
    const el = container.querySelector('[aria-hidden="true"]')
    expect(el?.className).toMatch(/oasis-bg-animate/)
  })
})
