import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { SecretInput } from './SecretInput'

describe('SecretInput', () => {
  it('never pre-fills a value that is already set, only hints at it via placeholder', () => {
    render(<SecretInput label="StashDB API key" isSet={true} value="" onChange={vi.fn()} />)

    const input = screen.getByLabelText<HTMLInputElement>('StashDB API key')
    expect(input.value).toBe('')
    expect(input.placeholder).toBe('Set — enter a new value to replace')
  })

  it('hints that an unset value is not configured and reports edits', () => {
    const onChange = vi.fn()
    render(<SecretInput label="AcoustID API key" isSet={false} value="" onChange={onChange} />)

    const input = screen.getByLabelText<HTMLInputElement>('AcoustID API key')
    expect(input.placeholder).toBe('Not set')

    fireEvent.change(input, { target: { value: 'new-key' } })
    expect(onChange).toHaveBeenCalledWith('new-key')
  })
})
