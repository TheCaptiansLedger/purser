import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { QueuePanel } from './QueuePanel'
import type { PlayerTrack } from '../lib/playerReducer'

const trackA: PlayerTrack = { id: 'a', title: 'Track A' }
const trackB: PlayerTrack = { id: 'b', title: 'Track B', artist: 'Artist B' }

describe('QueuePanel', () => {
  it('shows an empty-queue message when there are no upcoming tracks', () => {
    render(<QueuePanel queue={[]} onReorder={vi.fn()} onRemove={vi.fn()} />)
    expect(screen.getByText('Queue is empty.')).toBeInTheDocument()
  })

  it('lists every queued track, including artist when present', () => {
    render(<QueuePanel queue={[trackA, trackB]} onReorder={vi.fn()} onRemove={vi.fn()} />)
    expect(screen.getByText('Track A')).toBeInTheDocument()
    expect(screen.getByText('Track B — Artist B')).toBeInTheDocument()
  })

  it('calls onRemove with the clicked track id', () => {
    const onRemove = vi.fn()
    render(<QueuePanel queue={[trackA, trackB]} onReorder={vi.fn()} onRemove={onRemove} />)

    fireEvent.click(screen.getByRole('button', { name: 'Remove Track A from queue' }))
    expect(onRemove).toHaveBeenCalledWith('a')
  })

  it('calls onReorder with the moved array when a row is reordered via keyboard', () => {
    const onReorder = vi.fn()
    render(<QueuePanel queue={[trackA, trackB]} onReorder={onReorder} onRemove={vi.fn()} />)

    fireEvent.keyDown(screen.getByRole('button', { name: /Reorder Track A/ }), { key: 'ArrowDown' })
    expect(onReorder).toHaveBeenCalledWith([trackB, trackA])
  })
})
