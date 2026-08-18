import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { DropdownMenu } from './DropdownMenu'

describe('DropdownMenu', () => {
  it('opens on trigger click, closes on item select, and calls onSelect', () => {
    const onSelect = vi.fn()
    render(<DropdownMenu label="Add Thing" trigger="Add Thing" items={[{ label: 'Option A', onSelect }]} />)

    expect(screen.queryByRole('menu')).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Add Thing' }))
    expect(screen.getByRole('menu')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('menuitem', { name: 'Option A' }))
    expect(onSelect).toHaveBeenCalled()
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
  })

  it('closes on outside click and on Escape', () => {
    render(
      <div>
        <DropdownMenu label="Add Thing" trigger="Add Thing" items={[{ label: 'Option A', onSelect: () => {} }]} />
        <button type="button">Outside</button>
      </div>,
    )

    fireEvent.click(screen.getByRole('button', { name: 'Add Thing' }))
    expect(screen.getByRole('menu')).toBeInTheDocument()

    fireEvent.mouseDown(screen.getByRole('button', { name: 'Outside' }))
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Add Thing' }))
    expect(screen.getByRole('menu')).toBeInTheDocument()

    fireEvent.keyDown(document, { key: 'Escape' })
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
  })

  it('renders a disabled item as unclickable', () => {
    const onSelect = vi.fn()
    render(<DropdownMenu label="Add Thing" trigger="Add Thing" items={[{ label: 'Option A', onSelect, disabled: true }]} />)

    fireEvent.click(screen.getByRole('button', { name: 'Add Thing' }))
    expect(screen.getByRole('menuitem', { name: 'Option A' })).toBeDisabled()
  })
})
