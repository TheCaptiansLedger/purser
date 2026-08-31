import { describe, expect, it } from 'vitest'
import { initialPlayerState, playerReducer, type PlayerState, type PlayerTrack } from './playerReducer'

function track(id: string, overrides: Partial<PlayerTrack> = {}): PlayerTrack {
  return { id, title: `Track ${id}`, ...overrides }
}

describe('playerReducer', () => {
  it('PLAY sets current, starts playing, and resets position without touching the queue', () => {
    const state: PlayerState = { ...initialPlayerState, queue: [track('q1')], position: 42 }
    const next = playerReducer(state, { type: 'PLAY', track: track('a') })
    expect(next.current).toEqual(track('a'))
    expect(next.isPlaying).toBe(true)
    expect(next.position).toBe(0)
    expect(next.queue).toEqual([track('q1')])
  })

  it('ENQUEUE appends to the end of the queue', () => {
    const state: PlayerState = { ...initialPlayerState, queue: [track('a')] }
    const next = playerReducer(state, { type: 'ENQUEUE', track: track('b') })
    expect(next.queue).toEqual([track('a'), track('b')])
  })

  it('ENQUEUE_NEXT inserts at the front of the queue', () => {
    const state: PlayerState = { ...initialPlayerState, queue: [track('a')] }
    const next = playerReducer(state, { type: 'ENQUEUE_NEXT', track: track('b') })
    expect(next.queue).toEqual([track('b'), track('a')])
  })

  it('SET_QUEUE replaces the queue wholesale', () => {
    const state: PlayerState = { ...initialPlayerState, queue: [track('old')] }
    const next = playerReducer(state, { type: 'SET_QUEUE', tracks: [track('a'), track('b')] })
    expect(next.queue).toEqual([track('a'), track('b')])
  })

  it('SKIP_NEXT dequeues the head into current', () => {
    const state: PlayerState = {
      ...initialPlayerState,
      current: track('a'),
      queue: [track('b'), track('c')],
      isPlaying: false,
      position: 30,
    }
    const next = playerReducer(state, { type: 'SKIP_NEXT' })
    expect(next.current).toEqual(track('b'))
    expect(next.queue).toEqual([track('c')])
    expect(next.isPlaying).toBe(true)
    expect(next.position).toBe(0)
  })

  it('SKIP_NEXT with an empty queue clears current and stops playback', () => {
    const state: PlayerState = { ...initialPlayerState, current: track('a'), isPlaying: true, position: 15 }
    const next = playerReducer(state, { type: 'SKIP_NEXT' })
    expect(next.current).toBeNull()
    expect(next.isPlaying).toBe(false)
    expect(next.queue).toEqual([])
  })

  it('SKIP_PREVIOUS restarts the current track (no history stack in v1)', () => {
    const state: PlayerState = { ...initialPlayerState, current: track('a'), position: 50 }
    const next = playerReducer(state, { type: 'SKIP_PREVIOUS' })
    expect(next.current).toEqual(track('a'))
    expect(next.position).toBe(0)
  })

  it('PAUSE stops playback without clearing current', () => {
    const state: PlayerState = { ...initialPlayerState, current: track('a'), isPlaying: true }
    const next = playerReducer(state, { type: 'PAUSE' })
    expect(next.isPlaying).toBe(false)
    expect(next.current).toEqual(track('a'))
  })

  it('RESUME is a no-op with no current track', () => {
    const state = initialPlayerState
    const next = playerReducer(state, { type: 'RESUME' })
    expect(next).toBe(state)
  })

  it('RESUME starts playback when a current track exists', () => {
    const state: PlayerState = { ...initialPlayerState, current: track('a'), isPlaying: false }
    const next = playerReducer(state, { type: 'RESUME' })
    expect(next.isPlaying).toBe(true)
  })

  it('TOGGLE_PLAY flips isPlaying when a current track exists', () => {
    const playing: PlayerState = { ...initialPlayerState, current: track('a'), isPlaying: true }
    expect(playerReducer(playing, { type: 'TOGGLE_PLAY' }).isPlaying).toBe(false)

    const paused: PlayerState = { ...initialPlayerState, current: track('a'), isPlaying: false }
    expect(playerReducer(paused, { type: 'TOGGLE_PLAY' }).isPlaying).toBe(true)
  })

  it('TOGGLE_PLAY is a no-op with no current track', () => {
    const state = initialPlayerState
    expect(playerReducer(state, { type: 'TOGGLE_PLAY' })).toBe(state)
  })

  it('REMOVE_FROM_QUEUE drops only the matching track', () => {
    const state: PlayerState = { ...initialPlayerState, queue: [track('a'), track('b'), track('c')] }
    const next = playerReducer(state, { type: 'REMOVE_FROM_QUEUE', trackId: 'b' })
    expect(next.queue).toEqual([track('a'), track('c')])
  })

  it('REORDER_QUEUE moves a track from one index to another', () => {
    const state: PlayerState = { ...initialPlayerState, queue: [track('a'), track('b'), track('c')] }
    const next = playerReducer(state, { type: 'REORDER_QUEUE', fromIndex: 0, toIndex: 2 })
    expect(next.queue).toEqual([track('b'), track('c'), track('a')])
  })

  it('REORDER_QUEUE is a no-op for an out-of-range index', () => {
    const state: PlayerState = { ...initialPlayerState, queue: [track('a'), track('b')] }
    expect(playerReducer(state, { type: 'REORDER_QUEUE', fromIndex: 0, toIndex: 5 })).toBe(state)
    expect(playerReducer(state, { type: 'REORDER_QUEUE', fromIndex: -1, toIndex: 1 })).toBe(state)
    expect(playerReducer(state, { type: 'REORDER_QUEUE', fromIndex: 0, toIndex: 0 })).toBe(state)
  })

  it('SET_VOLUME clamps to [0, 1]', () => {
    expect(playerReducer(initialPlayerState, { type: 'SET_VOLUME', volume: 1.5 }).volume).toBe(1)
    expect(playerReducer(initialPlayerState, { type: 'SET_VOLUME', volume: -0.5 }).volume).toBe(0)
    expect(playerReducer(initialPlayerState, { type: 'SET_VOLUME', volume: 0.4 }).volume).toBe(0.4)
  })

  it('SET_POSITION updates position', () => {
    const next = playerReducer(initialPlayerState, { type: 'SET_POSITION', position: 12.5 })
    expect(next.position).toBe(12.5)
  })
})
