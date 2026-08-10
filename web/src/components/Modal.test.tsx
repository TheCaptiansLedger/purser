import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { Modal } from './Modal'

// Shared-component test per ADR 0004: exercised with two unrelated
// content configurations (a plain-text body, and a form-shaped body) to
// prove Modal carries no content-type knowledge of its own.
describe('Modal', () => {
  it('renders a title and arbitrary text content', () => {
    render(
      <Modal title="Job detail" onClose={vi.fn()}>
        <p>Kind: scan</p>
      </Modal>,
    )

    expect(screen.getByRole('dialog', { name: 'Job detail' })).toBeInTheDocument()
    expect(screen.getByText('Kind: scan')).toBeInTheDocument()
  })

  it('renders arbitrary interactive content', () => {
    render(
      <Modal title="Edit setting" onClose={vi.fn()}>
        <label>
          Value
          <input defaultValue="abc" />
        </label>
      </Modal>,
    )

    expect(screen.getByRole('dialog', { name: 'Edit setting' })).toBeInTheDocument()
    expect(screen.getByDisplayValue('abc')).toBeInTheDocument()
  })

  it('calls onClose when the close button is clicked', () => {
    const onClose = vi.fn()
    render(
      <Modal title="Job detail" onClose={onClose}>
        <p>body</p>
      </Modal>,
    )

    fireEvent.click(screen.getByRole('button', { name: 'Close' }))
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('calls onClose when the backdrop is clicked, but not when the dialog body is clicked', () => {
    const onClose = vi.fn()
    render(
      <Modal title="Job detail" onClose={onClose}>
        <p>body</p>
      </Modal>,
    )

    fireEvent.click(screen.getByText('body'))
    expect(onClose).not.toHaveBeenCalled()

    fireEvent.click(screen.getByRole('dialog').parentElement!)
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('calls onClose when Escape is pressed', () => {
    const onClose = vi.fn()
    render(
      <Modal title="Job detail" onClose={onClose}>
        <p>body</p>
      </Modal>,
    )

    fireEvent.keyDown(document, { key: 'Escape' })
    expect(onClose).toHaveBeenCalledTimes(1)
  })
})
