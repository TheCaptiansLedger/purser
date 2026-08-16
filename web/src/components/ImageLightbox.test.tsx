import { fireEvent, render, screen } from '@testing-library/react'
import { useState } from 'react'
import { describe, expect, it, vi } from 'vitest'
import { ImageLightbox } from './ImageLightbox'

// Shared-component test per ADR 0004: exercised with two unrelated
// content-type prop configurations (a Person photo and a Music cover) to
// prove ImageLightbox carries no content-type knowledge of its own.
describe('ImageLightbox', () => {
  it('renders the image with the given src and alt (person photo)', () => {
    render(<ImageLightbox src="/media/images/person-1" alt="Jane Doe portrait" onClose={vi.fn()} />)

    const img = screen.getByRole('img', { name: 'Jane Doe portrait' })
    expect(img).toHaveAttribute('src', '/media/images/person-1')
  })

  it('renders the image with the given src and alt (music cover)', () => {
    render(<ImageLightbox src="/media/images/cover-9" alt="Rumours cover art" onClose={vi.fn()} />)

    const img = screen.getByRole('img', { name: 'Rumours cover art' })
    expect(img).toHaveAttribute('src', '/media/images/cover-9')
  })

  it('calls onClose when Escape is pressed', () => {
    const onClose = vi.fn()
    render(<ImageLightbox src="/img.jpg" alt="cover" onClose={onClose} />)

    fireEvent.keyDown(document, { key: 'Escape' })
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('calls onClose when the backdrop is clicked, but not when the image is clicked', () => {
    const onClose = vi.fn()
    render(<ImageLightbox src="/img.jpg" alt="cover" onClose={onClose} />)

    fireEvent.click(screen.getByRole('img', { name: 'cover' }))
    expect(onClose).not.toHaveBeenCalled()

    fireEvent.click(screen.getByRole('dialog').parentElement!)
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('moves focus to the close button on open and traps Tab/Shift+Tab inside the overlay', () => {
    render(<ImageLightbox src="/img.jpg" alt="cover" onClose={vi.fn()} />)

    const closeButton = screen.getByRole('button', { name: 'Close' })
    expect(closeButton).toHaveFocus()

    fireEvent.keyDown(document, { key: 'Tab' })
    expect(closeButton).toHaveFocus()

    fireEvent.keyDown(document, { key: 'Tab', shiftKey: true })
    expect(closeButton).toHaveFocus()
  })

  it('restores focus to the trigger element when closed', () => {
    function Harness() {
      const [open, setOpen] = useState(false)
      return (
        <>
          <button onClick={() => setOpen(true)}>Open</button>
          {open && <ImageLightbox src="/img.jpg" alt="cover" onClose={() => setOpen(false)} />}
        </>
      )
    }

    render(<Harness />)
    const trigger = screen.getByRole('button', { name: 'Open' })
    trigger.focus()
    fireEvent.click(trigger)

    expect(screen.getByRole('button', { name: 'Close' })).toHaveFocus()

    fireEvent.keyDown(document, { key: 'Escape' })
    expect(trigger).toHaveFocus()
  })
})
