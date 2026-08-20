import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { SelectionToolbar } from './SelectionToolbar'

// ADR 0004: two content-type configurations (artists, albums) prove this
// bar carries no entity-specific branching.
describe('SelectionToolbar', () => {
  it('shows the live count with the given entity label, for artists', () => {
    render(<SelectionToolbar count={3} entityLabelPlural="artists" onDelete={vi.fn()} onCancel={vi.fn()} />)

    expect(screen.getByText('3 artists selected')).toBeInTheDocument()
  })

  it('shows the live count with the given entity label, for albums', () => {
    render(<SelectionToolbar count={1} entityLabelPlural="albums" onDelete={vi.fn()} onCancel={vi.fn()} />)

    expect(screen.getByText('1 albums selected')).toBeInTheDocument()
  })

  it('disables Delete at a zero count, and calls onDelete/onCancel otherwise', () => {
    const onDelete = vi.fn()
    const onCancel = vi.fn()
    const { rerender } = render(
      <SelectionToolbar count={0} entityLabelPlural="artists" onDelete={onDelete} onCancel={onCancel} />,
    )
    expect(screen.getByRole('button', { name: 'Delete' })).toBeDisabled()

    rerender(<SelectionToolbar count={2} entityLabelPlural="artists" onDelete={onDelete} onCancel={onCancel} />)
    fireEvent.click(screen.getByRole('button', { name: 'Delete' }))
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))

    expect(onDelete).toHaveBeenCalledOnce()
    expect(onCancel).toHaveBeenCalledOnce()
  })
})
