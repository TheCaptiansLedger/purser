import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { PersonCard } from './PersonCard'

// Shared-component test per ADR 0004: exercised with the two real
// consumer configurations — People index (no roles) and Artist Detail
// Members tab (role chips) — rendered directly, not asserted by
// inspection.
describe('PersonCard', () => {
  it('renders a People-index-style config: photo, name, no role chips', () => {
    render(<PersonCard person={{ id: 'p1', name: 'Jane Doe', imageId: 'img-1' }} />)

    expect(screen.getByText('Jane Doe')).toBeInTheDocument()
    const photoButton = screen.getByRole('button', { name: "View Jane Doe's photo" })
    expect(photoButton.querySelector('img')).toHaveAttribute('src', '/media/images/img-1')
    expect(screen.queryByText(/vocals|guitar/i)).not.toBeInTheDocument()
  })

  it('renders an Artist-Members-tab-style config: photo, name, multiple role chips', () => {
    render(
      <PersonCard
        person={{ id: 'p2', name: 'Stevie Nicks', imageId: 'img-2' }}
        roles={['Vocals', 'Former · 1970–1989']}
      />,
    )

    expect(screen.getByText('Stevie Nicks')).toBeInTheDocument()
    expect(screen.getByText('Vocals')).toBeInTheDocument()
    expect(screen.getByText('Former · 1970–1989')).toBeInTheDocument()
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
