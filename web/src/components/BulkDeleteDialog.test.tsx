import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { BulkDeleteDialog } from './BulkDeleteDialog'

// ADR 0004: two content-type configurations — a Group-shaped bulk delete
// (never blocking) and a LibraryEntry-shaped one (can block) — prove this
// dialog's blocking/cascade logic isn't hidden behind an entity-specific
// branch.
describe('BulkDeleteDialog', () => {
  it('shows a loading state while impact is still pending', () => {
    render(
      <BulkDeleteDialog
        entityLabelPlural="albums"
        count={2}
        impact={{ isPending: true, rows: [] }}
        onDelete={vi.fn()}
        isDeleting={false}
        onClose={vi.fn()}
      />,
    )

    expect(screen.getByText('Checking what references these albums…')).toBeInTheDocument()
  })

  it('albums (Group, never blocking): Delete is enabled with no cascade checkbox, and calls onDelete(false)', () => {
    const onDelete = vi.fn()
    render(
      <BulkDeleteDialog
        entityLabelPlural="albums"
        count={2}
        impact={{
          isPending: false,
          rows: [{ kind: 'item', label: 'Items (will be detached, not deleted)', count: 10, blocking: false }],
        }}
        onDelete={onDelete}
        isDeleting={false}
        onClose={vi.fn()}
      />,
    )

    expect(screen.getByText('Items (will be detached, not deleted)')).toBeInTheDocument()
    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument()
    const deleteButton = screen.getByRole('button', { name: 'Delete' })
    expect(deleteButton).not.toBeDisabled()

    fireEvent.click(deleteButton)
    expect(onDelete).toHaveBeenCalledWith(false)
  })

  it('artists (LibraryEntry, can block): Delete stays disabled until the cascade checkbox is checked, then calls onDelete(true)', () => {
    const onDelete = vi.fn()
    render(
      <BulkDeleteDialog
        entityLabelPlural="artists"
        count={1}
        impact={{
          isPending: false,
          rows: [
            { kind: 'group', label: 'Groups', count: 3, blocking: true },
            { kind: 'image', label: 'Images', count: 1, blocking: false },
          ],
        }}
        cascadeLabel="Also delete their albums and tracks"
        onDelete={onDelete}
        isDeleting={false}
        onClose={vi.fn()}
      />,
    )

    expect(screen.getByText('Groups')).toBeInTheDocument()
    expect(screen.getByText('Images')).toBeInTheDocument()
    const deleteButton = screen.getByRole('button', { name: 'Delete' })
    expect(deleteButton).toBeDisabled()

    fireEvent.click(screen.getByRole('checkbox'))
    expect(deleteButton).not.toBeDisabled()

    fireEvent.click(deleteButton)
    expect(onDelete).toHaveBeenCalledWith(true)
  })

  it('shows nothing-references and an inline error, without calling onClose, when the bulk delete fails', () => {
    const onClose = vi.fn()
    render(
      <BulkDeleteDialog
        entityLabelPlural="albums"
        count={1}
        impact={{ isPending: false, rows: [] }}
        onDelete={vi.fn()}
        isDeleting={false}
        deleteError="Couldn't delete these albums (unavailable)."
        onClose={onClose}
      />,
    )

    expect(screen.getByText('Nothing else references these albums.')).toBeInTheDocument()
    expect(screen.getByRole('alert')).toHaveTextContent("Couldn't delete these albums")
    expect(onClose).not.toHaveBeenCalled()
  })
})
