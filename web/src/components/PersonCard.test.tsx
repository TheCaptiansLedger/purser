import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { PersonCard } from './PersonCard'

// Shared-component test per ADR 0004: exercised with the real consumer
// configurations — People index (no roles), a plain read-only role chip,
// and Artist Detail Members tab's #724 actionable chip — rendered
// directly, not asserted by inspection.
describe('PersonCard', () => {
  it('renders a People-index-style config: photo, name, no role chips', () => {
    render(<PersonCard person={{ id: 'p1', name: 'Jane Doe', imageId: 'img-1' }} />)

    expect(screen.getByText('Jane Doe')).toBeInTheDocument()
    const photoButton = screen.getByRole('button', { name: "View Jane Doe's photo" })
    expect(photoButton.querySelector('img')).toHaveAttribute('src', '/media/images/img-1')
    expect(screen.queryByText(/vocals|guitar/i)).not.toBeInTheDocument()
  })

  it('renders plain, non-actionable role chips when no callbacks are given', () => {
    render(
      <PersonCard
        person={{ id: 'p2', name: 'Stevie Nicks', imageId: 'img-2' }}
        roles={[{ label: 'Vocals' }, { label: 'Former · 1970–1989' }]}
      />,
    )

    expect(screen.getByText('Stevie Nicks')).toBeInTheDocument()
    expect(screen.getByText('Vocals')).toBeInTheDocument()
    expect(screen.getByText('Former · 1970–1989')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Edit|Remove/ })).not.toBeInTheDocument()
  })

  it('renders an Artist-Members-tab-style config: an actionable chip with Edit/Remove icons', () => {
    const onEdit = vi.fn()
    const onRemove = vi.fn()
    render(
      <PersonCard
        person={{ id: 'p2', name: 'Stevie Nicks', imageId: 'img-2' }}
        roles={[{ label: 'vocalist (since 1975)', id: 'vocalist', onEdit, onRemove }]}
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: "Edit Stevie Nicks's vocalist role" }))
    expect(onEdit).toHaveBeenCalledTimes(1)

    fireEvent.click(screen.getByRole('button', { name: "Remove Stevie Nicks's vocalist role" }))
    expect(onRemove).toHaveBeenCalledTimes(1)
  })

  it('renders a placeholder avatar when no imageId is given', () => {
    render(<PersonCard person={{ id: 'p3', name: 'No Photo' }} />)

    expect(screen.getByText('No Photo')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /photo/i })).not.toBeInTheDocument()
  })

  it('opens the lightbox on photo click and closes it on Escape', () => {
    render(<PersonCard person={{ id: 'p1', name: 'Jane Doe', imageId: 'img-1' }} />)

    fireEvent.click(screen.getByRole('button', { name: "View Jane Doe's photo" }))
    expect(screen.getByRole('dialog', { name: 'Jane Doe' })).toBeInTheDocument()

    fireEvent.keyDown(document, { key: 'Escape' })
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })
})
