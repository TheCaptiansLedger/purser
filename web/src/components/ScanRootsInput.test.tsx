import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { ScanRootsInput } from './ScanRootsInput'

describe('ScanRootsInput', () => {
  it('renders each row as a real path input and content-type select, not stringified text', () => {
    const value = [
      { path: '/media/incoming', content_type: 'music' },
      { path: '/media/movies-in', content_type: 'movie' },
    ]
    render(<ScanRootsInput label="Scan Roots" value={value} onChange={vi.fn()} />)

    expect(screen.getByLabelText<HTMLInputElement>('Scan Roots path 1').value).toBe('/media/incoming')
    expect(screen.getByLabelText<HTMLSelectElement>('Scan Roots content type 1').value).toBe('music')
    expect(screen.getByLabelText<HTMLInputElement>('Scan Roots path 2').value).toBe('/media/movies-in')
    expect(screen.queryByText('[object Object]')).not.toBeInTheDocument()
  })

  it('edits a row path in place without touching the other rows', () => {
    const onChange = vi.fn()
    const value = [{ path: '/media/incoming', content_type: 'music' }]
    render(<ScanRootsInput label="Scan Roots" value={value} onChange={onChange} />)

    fireEvent.change(screen.getByLabelText('Scan Roots path 1'), { target: { value: '/media/new-incoming' } })
    expect(onChange).toHaveBeenCalledWith([{ path: '/media/new-incoming', content_type: 'music' }])
  })

  it('adds a new path/content-type row and removes an existing one', () => {
    const onChange = vi.fn()
    const { rerender } = render(<ScanRootsInput label="Scan Roots" value={[]} onChange={onChange} />)

    fireEvent.change(screen.getByLabelText('Add to Scan Roots'), { target: { value: '/media/books-in' } })
    fireEvent.change(screen.getByLabelText('Content type for new Scan Roots entry'), { target: { value: 'book' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))
    expect(onChange).toHaveBeenCalledWith([{ path: '/media/books-in', content_type: 'book' }])

    rerender(<ScanRootsInput label="Scan Roots" value={[{ path: '/media/books-in', content_type: 'book' }]} onChange={onChange} />)
    fireEvent.click(screen.getByRole('button', { name: 'Remove /media/books-in' }))
    expect(onChange).toHaveBeenCalledWith([])
  })

  it('hides all edit affordances when disabled', () => {
    const value = [{ path: '/media/incoming', content_type: 'music' }]
    render(<ScanRootsInput label="Scan Roots" value={value} onChange={vi.fn()} disabled />)

    expect(screen.getByLabelText('Scan Roots path 1')).toBeDisabled()
    expect(screen.queryByLabelText('Add to Scan Roots')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Remove/ })).not.toBeInTheDocument()
  })
})
