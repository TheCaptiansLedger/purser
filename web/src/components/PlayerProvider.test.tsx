import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { PlayerProvider } from './PlayerProvider'
import { usePlayer } from '../hooks/usePlayer'
import type { PlayerTrack } from '../lib/playerReducer'

// jsdom doesn't implement HTMLMediaElement.play/pause/load — stubbed here
// so PlayerProvider's effects (which call them) don't throw "Not
// implemented" in every test.
beforeEach(() => {
  window.HTMLMediaElement.prototype.play = vi.fn().mockResolvedValue(undefined)
  window.HTMLMediaElement.prototype.pause = vi.fn()
})

const trackA: PlayerTrack = { id: 'a', title: 'Track A' }
const trackB: PlayerTrack = { id: 'b', title: 'Track B' }

// TestConsumer exercises usePlayer the way a real page/component would —
// PlayerProvider has no UI of its own to assert against otherwise.
function TestConsumer() {
  const { state, dispatch } = usePlayer()
  return (
    <div>
      <span data-testid="current">{state.current?.title ?? 'none'}</span>
      <span data-testid="is-playing">{String(state.isPlaying)}</span>
      <span data-testid="queue-length">{state.queue.length}</span>
      <button onClick={() => dispatch({ type: 'PLAY', track: trackA })}>play a</button>
      <button onClick={() => dispatch({ type: 'ENQUEUE', track: trackB })}>enqueue b</button>
      <button onClick={() => dispatch({ type: 'PAUSE' })}>pause</button>
    </div>
  )
}

function renderWithProvider() {
  return render(
    <PlayerProvider>
      <TestConsumer />
    </PlayerProvider>,
  )
}

describe('PlayerProvider', () => {
  it('renders an <audio> element alongside children', () => {
    const { container } = renderWithProvider()
    expect(container.querySelector('audio')).not.toBeNull()
  })

  it('dispatching PLAY updates state and calls audio.play()', async () => {
    renderWithProvider()
    fireEvent.click(screen.getByText('play a'))

    expect(screen.getByTestId('current').textContent).toBe('Track A')
    expect(screen.getByTestId('is-playing').textContent).toBe('true')
    await waitFor(() => expect(window.HTMLMediaElement.prototype.play).toHaveBeenCalled())
  })

  it('sets the audio element src to /media/audio/{trackId} when current changes', async () => {
    const { container } = renderWithProvider()
    fireEvent.click(screen.getByText('play a'))

    await waitFor(() => {
      const audio = container.querySelector('audio')
      expect(audio?.getAttribute('src')).toBe('/media/audio/a')
    })
  })

  it('dispatching PAUSE calls audio.pause() and updates state', async () => {
    renderWithProvider()
    fireEvent.click(screen.getByText('play a'))
    await waitFor(() => expect(screen.getByTestId('is-playing').textContent).toBe('true'))

    fireEvent.click(screen.getByText('pause'))
    expect(screen.getByTestId('is-playing').textContent).toBe('false')
    expect(window.HTMLMediaElement.prototype.pause).toHaveBeenCalled()
  })

  it('ENQUEUE updates queue length without affecting current', () => {
    renderWithProvider()
    fireEvent.click(screen.getByText('enqueue b'))
    expect(screen.getByTestId('queue-length').textContent).toBe('1')
    expect(screen.getByTestId('current').textContent).toBe('none')
  })

  it("SKIP_NEXT via the audio element's ended event advances to the next queued track", async () => {
    const { container } = renderWithProvider()
    fireEvent.click(screen.getByText('play a'))
    fireEvent.click(screen.getByText('enqueue b'))
    await waitFor(() => expect(screen.getByTestId('current').textContent).toBe('Track A'))

    const audio = container.querySelector('audio')
    audio?.dispatchEvent(new Event('ended'))

    await waitFor(() => expect(screen.getByTestId('current').textContent).toBe('Track B'))
    expect(screen.getByTestId('queue-length').textContent).toBe('0')
  })
})
