import { createContext, useEffect, useReducer, useRef, type ReactNode } from 'react'
import { initialPlayerState, playerReducer, type PlayerAction, type PlayerState } from '../lib/playerReducer'

export interface PlayerContextValue {
  state: PlayerState
  dispatch: React.Dispatch<PlayerAction>
}

// PlayerContext has no default value — usePlayer (hooks/usePlayer.ts)
// throws when used outside PlayerProvider rather than silently falling
// back to a disconnected state, same "fail loud, not silent" convention
// this codebase's other required-context hooks follow.
export const PlayerContext = createContext<PlayerContextValue | null>(null)

// PlayerProvider is #749's side-effecting half: playerReducer only computes
// *what* should be playing, this component is what makes an actual <audio>
// element do it. Mounted once in Layout.tsx so both the element and the
// state survive route navigation instead of remounting per page.
//
// The endpoint built in #748 (GET /media/audio/{itemId}) is same-origin —
// same relative-path convention /media/images/{id} already uses (see
// PersonCard/ArtistCard/AlbumCard), no transport/baseUrl change needed.
export function PlayerProvider({ children }: { children: ReactNode }) {
  const [state, dispatch] = useReducer(playerReducer, initialPlayerState)
  const audioRef = useRef<HTMLAudioElement>(null)

  // src follows current — a track change loads a new file. Deliberately
  // keyed on current?.id rather than the whole current object, so a
  // reducer action that replaces `current` with an equal-but-new object
  // reference (there isn't one today, but nothing enforces it) doesn't
  // reload the element mid-playback.
  useEffect(() => {
    const audio = audioRef.current
    if (!audio || !state.current) {
      return
    }
    audio.src = `/media/audio/${state.current.id}`
  }, [state.current?.id])

  useEffect(() => {
    const audio = audioRef.current
    if (!audio) {
      return
    }
    if (state.isPlaying) {
      // play() returns a Promise that rejects if interrupted (e.g. a new
      // src loads before this one starts) — not a real error, so it's
      // swallowed rather than surfaced.
      void audio.play().catch(() => {})
    } else {
      audio.pause()
    }
  }, [state.isPlaying, state.current?.id])

  useEffect(() => {
    const audio = audioRef.current
    if (audio) {
      audio.volume = state.volume
    }
  }, [state.volume])

  return (
    <PlayerContext.Provider value={{ state, dispatch }}>
      {children}
      <audio
        ref={audioRef}
        onEnded={() => dispatch({ type: 'SKIP_NEXT' })}
        onTimeUpdate={e => dispatch({ type: 'SET_POSITION', position: e.currentTarget.currentTime })}
      />
    </PlayerContext.Provider>
  )
}
