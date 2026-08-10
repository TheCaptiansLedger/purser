import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { ListInput } from './ListInput'

describe('ListInput', () => {
  it('adds a trimmed entry to the end, preserving existing order', () => {
    const onChange = vi.fn()
    render(<ListInput label="Movies roots" value={['/media/movies']} onChange={onChange} />)

    fireEvent.change(screen.getByLabelText('Add to Movies roots'), { target: { value: '  /media/movies2  ' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    expect(onChange).toHaveBeenCalledWith(['/media/movies', '/media/movies2'])
  })

  it('removes an entry by index and hides the add row when disabled', () => {
    const onChange = vi.fn()
    render(<ListInput label="AfterDark provider priority" value={['stashdb', 'tpdb']} onChange={onChange} disabled />)

    expect(screen.queryByLabelText('Add to AfterDark provider priority')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('Remove stashdb')).not.toBeInTheDocument()
    expect(screen.getByText('stashdb')).toBeInTheDocument()
    expect(screen.getByText('tpdb')).toBeInTheDocument()
  })

  it('removes the item at the clicked index when not disabled', () => {
    const onChange = vi.fn()
    render(<ListInput label="AfterDark provider priority" value={['stashdb', 'tpdb']} onChange={onChange} />)

    fireEvent.click(screen.getByLabelText('Remove stashdb'))
    expect(onChange).toHaveBeenCalledWith(['tpdb'])
  })
})
