import { fireEvent, render, screen } from '@testing-library/react'
import { UserPlus, SearchX } from 'lucide-react'
import { describe, expect, it, vi } from 'vitest'
import { EmptyState } from './EmptyState'

// Shared-component test per ADR 0004: two real configurations — an
// empty-library state with a next action (People index, #660), and a
// zero-results state with no action at all (search/filter yielded
// nothing on a non-empty library).
describe('EmptyState', () => {
  it('renders an empty-library config with an enabled action', () => {
    const onClick = vi.fn()
    render(
      <EmptyState
        icon={UserPlus}
        title="No people yet"
        description="Add a person to start building your library."
        action={{ label: 'Add Person', onClick }}
      />,
    )

    expect(screen.getByText('No people yet')).toBeInTheDocument()
    const button = screen.getByRole('button', { name: 'Add Person' })
    fireEvent.click(button)
    expect(onClick).toHaveBeenCalledOnce()
  })

  it('renders a disabled-action config, not calling onClick when clicked', () => {
    const onClick = vi.fn()
    render(
      <EmptyState
        icon={UserPlus}
        title="No people yet"
        description="Add a person to start building your library."
        action={{ label: 'Add Person', onClick, disabled: true }}
      />,
    )

    const button = screen.getByRole('button', { name: 'Add Person' })
    expect(button).toBeDisabled()
    fireEvent.click(button)
    expect(onClick).not.toHaveBeenCalled()
  })

  it('renders a zero-results config with no action button at all', () => {
    render(
      <EmptyState
        icon={SearchX}
        title="No matches"
        description="No people match your search or filter."
      />,
    )

    expect(screen.getByText('No matches')).toBeInTheDocument()
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })
})
