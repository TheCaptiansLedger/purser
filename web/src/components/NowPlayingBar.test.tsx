import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { NowPlayingBar } from './NowPlayingBar'
import { PlayerProvider } from './PlayerProvider'
import { usePlayer } from '../hooks/usePlayer'
import type { PlayerTrack } from '../lib/playerReducer'

const trackA: PlayerTrack = { id: 'a', title: 'Track A', artist: 'Artist A', runtimeSeconds: 125 }
const trackB: PlayerTrack = { id: 'b', title: 'Track B' }

// TestHarness dispatches directly, the same way a future consumer
// (Tracklist, #751) will — NowPlayingBar itself only ever reads state.
function TestHarness() {
  const { dispatch } = usePlayer()
  return (
    <>
      <button onClick={() => dispatch({ type: 'PLAY', track: trackA })}>play a</button>
      <button onClick={() => dispatch({ type: 'ENQUEUE', track: trackB })}>enqueue b</button>
      <NowPlayingBar />
    </>
  )
}

function renderBar() {
  return render(
    <PlayerProvider>
      <TestHarness />
    </PlayerProvider>,
  )
}

describe('NowPlayingBar', () => {
  it('renders nothing when no track has ever been played', () => {
    renderBar()
    expect(screen.queryByRole('button', { name: 'Play' })).not.toBeInTheDocument()
  })

  it('shows title, artist, and total runtime once a track is playing', () => {
    renderBar()
    fireEvent.click(screen.getByText('play a'))

    expect(screen.getByText('Track A')).toBeInTheDocument()
    expect(screen.getByText('Artist A')).toBeInTheDocument()
    expect(screen.getByText('0:00 / 2:05')).toBeInTheDocument()
  })

  it('toggles between Play and Pause labels', () => {
    renderBar()
    fireEvent.click(screen.getByText('play a'))

    expect(screen.getByRole('button', { name: 'Pause' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Pause' }))
    expect(screen.getByRole('button', { name: 'Play' })).toBeInTheDocument()
  })

  it('skip-next is disabled with an empty queue and enabled once something is queued', () => {
    renderBar()
    fireEvent.click(screen.getByText('play a'))

    expect(screen.getByRole('button', { name: 'Skip to next track' })).toBeDisabled()

    fireEvent.click(screen.getByText('enqueue b'))
    expect(screen.getByRole('button', { name: 'Skip to next track' })).not.toBeDisabled()
  })

  it('skip-next advances to the next queued track', () => {
    renderBar()
    fireEvent.click(screen.getByText('play a'))
    fireEvent.click(screen.getByText('enqueue b'))

    fireEvent.click(screen.getByRole('button', { name: 'Skip to next track' }))
    expect(screen.getByText('Track B')).toBeInTheDocument()
  })

  it('skip-previous restarts the current track', () => {
    renderBar()
    fireEvent.click(screen.getByText('play a'))

    fireEvent.click(screen.getByRole('button', { name: 'Skip to previous track' }))
    expect(screen.getByText('Track A')).toBeInTheDocument()
  })

  it('toggles the queue panel open and closed, and reflects queued tracks', () => {
    renderBar()
    fireEvent.click(screen.getByText('play a'))
    fireEvent.click(screen.getByText('enqueue b'))

    expect(screen.queryByText('Up next')).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Toggle queue' }))
    expect(screen.getByText('Up next')).toBeInTheDocument()
    expect(screen.getByText('Track B')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Toggle queue' }))
    expect(screen.queryByText('Up next')).not.toBeInTheDocument()
  })

  it('removing a track from the queue panel updates player state', () => {
    renderBar()
    fireEvent.click(screen.getByText('play a'))
    fireEvent.click(screen.getByText('enqueue b'))
    fireEvent.click(screen.getByRole('button', { name: 'Toggle queue' }))

    fireEvent.click(screen.getByRole('button', { name: 'Remove Track B from queue' }))

    expect(screen.getByText('Queue is empty.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Skip to next track' })).toBeDisabled()
  })

  it('closes the queue panel on outside click', () => {
    renderBar()
    fireEvent.click(screen.getByText('play a'))
    fireEvent.click(screen.getByRole('button', { name: 'Toggle queue' }))
    expect(screen.getByText('Up next')).toBeInTheDocument()

    fireEvent.mouseDown(document.body)
    expect(screen.queryByText('Up next')).not.toBeInTheDocument()
  })
})
