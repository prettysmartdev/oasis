import { render, screen } from '@testing-library/react'
import HomePage from '../app/page'

describe('HomePage', () => {
  it('renders the oasis heading', () => {
    render(<HomePage />)
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('oasis')
  })
})
