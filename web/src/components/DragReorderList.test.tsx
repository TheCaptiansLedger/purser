import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { DragReorderList } from './DragReorderList'

describe('DragReorderList', () => {
  it('reorders string items via ArrowDown on the focused handle (keyboard path)', () => {
    const onReorder = vi.fn()
    render(
      <DragReorderList
        items={['stashdb', 'tpdb']}
        getKey={v => v}
        getLabel={v => v}
        renderItem={v => v}
        onReorder={onReorder}
        ariaLabel="Provider priority"
      />,
    )

    fireEvent.keyDown(screen.getByRole('button', { name: 'Reorder stashdb, position 1 of 2' }), { key: 'ArrowDown' })
    expect(onReorder).toHaveBeenCalledWith(['tpdb', 'stashdb'])
  })

  it('reorders object items via native drag-and-drop (pointer path), proving it is generic over T', () => {
    const onReorder = vi.fn()
    const items = [
      { path: '/media/a', content_type: 'movie' },
      { path: '/media/b', content_type: 'tv' },
    ]
    render(
      <DragReorderList
        items={items}
        getKey={item => item.path}
        renderItem={item => item.path}
        onReorder={onReorder}
        ariaLabel="Scan roots"
      />,
    )

    const rows = screen.getAllByRole('listitem')
    fireEvent.dragStart(rows[0])
    fireEvent.dragOver(rows[1])
    fireEvent.drop(rows[1])

    expect(onReorder).toHaveBeenCalledWith([items[1], items[0]])
  })

  it('ignores ArrowUp on the first row and ArrowDown on the last row (no out-of-bounds move)', () => {
    const onReorder = vi.fn()
    render(
      <DragReorderList items={['a', 'b']} getKey={v => v} renderItem={v => v} onReorder={onReorder} ariaLabel="List" />,
    )

    fireEvent.keyDown(screen.getByRole('button', { name: 'Reorder a, position 1 of 2' }), { key: 'ArrowUp' })
    fireEvent.keyDown(screen.getByRole('button', { name: 'Reorder b, position 2 of 2' }), { key: 'ArrowDown' })
    expect(onReorder).not.toHaveBeenCalled()
  })

  it('disables both drag and keyboard reordering when disabled', () => {
    const onReorder = vi.fn()
    render(
      <DragReorderList
        items={['a', 'b']}
        getKey={v => v}
        renderItem={v => v}
        onReorder={onReorder}
        ariaLabel="List"
        disabled
      />,
    )

    const handle = screen.getByRole('button', { name: 'Reorder a, position 1 of 2' })
    expect(handle).toBeDisabled()
    fireEvent.keyDown(handle, { key: 'ArrowDown' })
    expect(onReorder).not.toHaveBeenCalled()
  })
})
