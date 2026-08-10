import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { TextInput } from './TextInput'

describe('TextInput', () => {
  it('reports a plain string for the default text type', () => {
    const onChange = vi.fn()
    render(<TextInput label="Media path" value="/data/media" onChange={onChange} />)

    const input = screen.getByLabelText('Media path')
    fireEvent.change(input, { target: { value: '/data/media2' } })
    expect(onChange).toHaveBeenCalledWith('/data/media2')
  })

  it('reports a parsed number for the number type', () => {
    const onChange = vi.fn()
    render(<TextInput label="Confidence threshold" value={0.8} onChange={onChange} type="number" />)

    const input = screen.getByLabelText('Confidence threshold')
    fireEvent.change(input, { target: { value: '0.9' } })
    expect(onChange).toHaveBeenCalledWith(0.9)
  })
})
