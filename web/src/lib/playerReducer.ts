// PlayerTrack is the in-browser player's queue-entry shape — deliberately
// its own type, not the generated Item Message (web/src/gen/purser/domain/v1/item_pb):
// the player only ever needs id (to build /media/audio/{id}, see #748),
// title, and the display fields below, and shouldn't carry proto wire
// plumbing (oneofs, method descriptors) through every reducer action.
// Callers (Tracklist et al., wired in #751) map an Item to a PlayerTrack at
// the point they enqueue it.
export interface PlayerTrack {
  id: string
  title: string
  artist?: string
  runtimeSeconds?: number
}

export interface PlayerState {
  // queue is strictly "up next" — it never includes current. Advancing
  // (SKIP_NEXT, or the <audio> element's 'ended' event in PlayerProvider)
  // dequeues its head into current.
  queue: PlayerTrack[]
  current: PlayerTrack | null
  isPlaying: boolean
  position: number
  volume: number
}

export const initialPlayerState: PlayerState = {
  queue: [],
  current: null,
  isPlaying: false,
  position: 0,
  volume: 1,
}

// PlayerAction — PLAY and SET_QUEUE are deliberately separate primitives
// rather than one "play this list" action: "play this one track now" and
// "replace what's queued next" are different concerns a caller composes
// (e.g. "play this whole album" dispatches SET_QUEUE with the remaining
// tracks, then PLAY with the first) rather than the reducer guessing intent
// from a combined payload.
export type PlayerAction =
  | { type: 'PLAY'; track: PlayerTrack }
  | { type: 'ENQUEUE'; track: PlayerTrack }
  | { type: 'ENQUEUE_NEXT'; track: PlayerTrack }
  | { type: 'SET_QUEUE'; tracks: PlayerTrack[] }
  | { type: 'SKIP_NEXT' }
  | { type: 'SKIP_PREVIOUS' }
  | { type: 'PAUSE' }
  | { type: 'RESUME' }
  | { type: 'TOGGLE_PLAY' }
  | { type: 'REMOVE_FROM_QUEUE'; trackId: string }
  | { type: 'REORDER_QUEUE'; fromIndex: number; toIndex: number }
  | { type: 'SET_VOLUME'; volume: number }
  | { type: 'SET_POSITION'; position: number }

// playerReducer is pure and has no idea an <audio> element exists —
// PlayerProvider is the side-effecting half that reacts to isPlaying/
// current/volume changes by actually calling .play()/.pause() and setting
// .src, and reports position/ended back in via SET_POSITION/SKIP_NEXT.
export function playerReducer(state: PlayerState, action: PlayerAction): PlayerState {
  switch (action.type) {
    case 'PLAY':
      return { ...state, current: action.track, isPlaying: true, position: 0 }

    case 'ENQUEUE':
      return { ...state, queue: [...state.queue, action.track] }

    case 'ENQUEUE_NEXT':
      return { ...state, queue: [action.track, ...state.queue] }

    case 'SET_QUEUE':
      return { ...state, queue: action.tracks }

    case 'SKIP_NEXT': {
      const [next, ...rest] = state.queue
      if (!next) {
        return { ...state, current: null, isPlaying: false, position: 0 }
      }
      return { ...state, current: next, queue: rest, isPlaying: true, position: 0 }
    }

    // v1: no play-history stack, so "previous" restarts the current track
    // rather than actually going back — a real back-stack is future scope,
    // not something #749's plumbing-only pass needs to solve.
    case 'SKIP_PREVIOUS':
      return { ...state, position: 0 }

    case 'PAUSE':
      return { ...state, isPlaying: false }

    case 'RESUME':
      return state.current ? { ...state, isPlaying: true } : state

    case 'TOGGLE_PLAY':
      return state.current ? { ...state, isPlaying: !state.isPlaying } : state

    case 'REMOVE_FROM_QUEUE':
      return { ...state, queue: state.queue.filter(t => t.id !== action.trackId) }

    case 'REORDER_QUEUE': {
      const { fromIndex, toIndex } = action
      if (
        fromIndex === toIndex ||
        fromIndex < 0 ||
        toIndex < 0 ||
        fromIndex >= state.queue.length ||
        toIndex >= state.queue.length
      ) {
        return state
      }
      const queue = [...state.queue]
      const [moved] = queue.splice(fromIndex, 1)
      queue.splice(toIndex, 0, moved)
      return { ...state, queue }
    }

    case 'SET_VOLUME':
      return { ...state, volume: Math.min(1, Math.max(0, action.volume)) }

    case 'SET_POSITION':
      return { ...state, position: action.position }

    default:
      return state
  }
}
