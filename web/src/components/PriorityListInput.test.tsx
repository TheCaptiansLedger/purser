import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { PriorityListInput } from './PriorityListInput'

const AFTERDARK_OPTIONS = [
  { value: 'stashdb', label: 'StashDB' },
  { value: 'tpdb', label: 'ThePornDB' },
]

describe('PriorityListInput', () => {
  it('renders each value by its option label, numbered in rank order', () => {
    render(
      <PriorityListInput label="Provider Priority" value={['stashdb', 'tpdb']} options={AFTERDARK_OPTIONS} onChange={vi.fn()} />,
    )

    expect(screen.getByText('StashDB')).toBeInTheDocument()
    expect(screen.getByText('ThePornDB')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Reorder StashDB, position 1 of 2' })).toBeInTheDocument()
  })

  it('reorders via the keyboard path and reports the new value array, not a label', () => {
    const onChange = vi.fn()
    render(
      <PriorityListInput label="Provider Priority" value={['stashdb', 'tpdb']} options={AFTERDARK_OPTIONS} onChange={onChange} />,
    )

    fireEvent.keyDown(screen.getByRole('button', { name: 'Reorder StashDB, position 1 of 2' }), { key: 'ArrowDown' })
    expect(onChange).toHaveBeenCalledWith(['tpdb', 'stashdb'])
  })

  it('renders a placeholder instead of an empty list when value has no entries', () => {
    render(<PriorityListInput label="Provider Priority" value={[]} options={AFTERDARK_OPTIONS} onChange={vi.fn()} />)
    expect(screen.getByText('—')).toBeInTheDocument()
    expect(screen.queryByRole('list')).not.toBeInTheDocument()
  })

  it('falls back to the raw value when an option has no matching label, proving it never invents free text', () => {
    render(
      <PriorityListInput label="Provider Priority" value={['unknownprovider']} options={AFTERDARK_OPTIONS} onChange={vi.fn()} />,
    )
    expect(screen.getByText('unknownprovider')).toBeInTheDocument()
  })
})
