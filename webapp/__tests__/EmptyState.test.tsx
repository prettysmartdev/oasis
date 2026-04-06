import React from 'react'
import { render, screen } from '@testing-library/react'
import EmptyState from '@/components/EmptyState'

describe('EmptyState', () => {
  describe('agents page', () => {
    it('renders correct headline', () => {
      render(<EmptyState page="agents" />)
      expect(screen.getByRole('heading', { name: /Your OaSis awaits/i })).toBeInTheDocument()
    })

    it('renders agents-specific subline', () => {
      render(<EmptyState page="agents" />)
      expect(screen.getByText(/Add your first agent from the terminal/i)).toBeInTheDocument()
    })

    it('code block contains oasis app add', () => {
      render(<EmptyState page="agents" />)
      expect(screen.getByText('oasis app add')).toBeInTheDocument()
    })
  })

  describe('apps page', () => {
    it('renders correct headline', () => {
      render(<EmptyState page="apps" />)
      expect(screen.getByRole('heading', { name: /Your OaSis awaits/i })).toBeInTheDocument()
    })

    it('renders apps-specific subline', () => {
      render(<EmptyState page="apps" />)
      expect(screen.getByText(/Add your first app from the terminal/i)).toBeInTheDocument()
    })

    it('code block contains oasis app add', () => {
      render(<EmptyState page="apps" />)
      expect(screen.getByText('oasis app add')).toBeInTheDocument()
    })
  })

  it('code block uses mono font', () => {
    const { container } = render(<EmptyState page="apps" />)
    const pre = container.querySelector('pre')
    expect(pre).toBeInTheDocument()
    expect(pre).toHaveClass('font-mono')
  })
})
