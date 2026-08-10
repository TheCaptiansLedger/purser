import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { Toggle } from './Toggle'

describe('Toggle', () => {
  it('reflects an unchecked, enabled config and reports a click', () => {
    const onChange = vi.fn()
    render(<Toggle label="Enable movies module" checked={false} onChange={onChange} />)

    const toggle = screen.getByRole('switch', { name: 'Enable movies module' })
    expect(toggle).toHaveAttribute('aria-checked', 'false')

    fireEvent.click(toggle)
    expect(onChange).toHaveBeenCalledWith(true)
  })

  it('reflects a checked, disabled config and ignores a click', () => {
    const onChange = vi.fn()
    render(<Toggle label="Enable AfterDark module" checked={true} onChange={onChange} disabled />)

    const toggle = screen.getByRole('switch', { name: 'Enable AfterDark module' })
    expect(toggle).toHaveAttribute('aria-checked', 'true')
    expect(toggle).toBeDisabled()

    fireEvent.click(toggle)
    expect(onChange).not.toHaveBeenCalled()
  })
})
