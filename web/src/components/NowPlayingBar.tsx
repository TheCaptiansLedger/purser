import { useEffect } from 'react'
import { Pause, Play, SkipBack, SkipForward } from 'lucide-react'
import { usePlayer } from '../hooks/usePlayer'
import { formatRuntime } from '../lib/formatRuntime'

// PLAYER_BAR_HEIGHT mirrors Layout.tsx's own --sidebar-width convention:
// a CSS custom property this component sets on <html> while visible, so
// Layout's <main> can reserve exactly enough bottom padding to not have
// its content hidden behind the fixed bar — without Layout itself needing
// to know whether a track is playing (it isn't a PlayerContext consumer).
const PLAYER_BAR_HEIGHT = '4rem'
const PLAYER_BAR_HEIGHT_VAR = '--player-bar-height'

// NowPlayingBar is #750: a persistent play/pause/skip bar for the player
// state PlayerProvider (#749) already tracks. Deliberately scoped to
// exactly that — no seek bar, no volume control (#750's own scope note) —
// those are separate future work if/when wanted, not silently folded in
// here.
export function NowPlayingBar() {
  const { state, dispatch } = usePlayer()
  const visible = state.current !== null

  useEffect(() => {
    document.documentElement.style.setProperty(PLAYER_BAR_HEIGHT_VAR, visible ? PLAYER_BAR_HEIGHT : '0px')
    return () => {
      document.documentElement.style.setProperty(PLAYER_BAR_HEIGHT_VAR, '0px')
    }
  }, [visible])

  if (!state.current) {
    return null
  }

  const elapsed = formatRuntime(Math.floor(state.position)) ?? '0:00'
  const total = state.current.runtimeSeconds ? formatRuntime(state.current.runtimeSeconds) : undefined

  return (
    <div
      className="fixed bottom-0 left-0 right-0 z-40 flex h-16 items-center gap-4 border-t border-border bg-surface/95 px-4 backdrop-blur-xl"
      style={{ paddingLeft: 'calc(var(--sidebar-width, 0px) + 1rem)' }}
    >
      <div className="min-w-0 flex-1">
        <p className="truncate text-body font-medium text-text">{state.current.title}</p>
        {state.current.artist && <p className="truncate text-label text-text-secondary">{state.current.artist}</p>}
      </div>

      <span className="shrink-0 text-label text-text-secondary">
        {elapsed}
        {total ? ` / ${total}` : ''}
      </span>

      <div className="flex shrink-0 items-center gap-2">
        <button
          type="button"
          onClick={() => dispatch({ type: 'SKIP_PREVIOUS' })}
          aria-label="Skip to previous track"
          title="Previous"
          className="flex h-8 w-8 items-center justify-center rounded-lg text-text-secondary hover:bg-surface-raised hover:text-text"
        >
          <SkipBack size={16} />
        </button>
        <button
          type="button"
          onClick={() => dispatch({ type: 'TOGGLE_PLAY' })}
          aria-label={state.isPlaying ? 'Pause' : 'Play'}
          title={state.isPlaying ? 'Pause' : 'Play'}
          className="flex h-9 w-9 items-center justify-center rounded-full bg-accent-system text-bg hover:opacity-90"
        >
          {state.isPlaying ? <Pause size={18} /> : <Play size={18} />}
        </button>
        <button
          type="button"
          onClick={() => dispatch({ type: 'SKIP_NEXT' })}
          disabled={state.queue.length === 0}
          aria-label="Skip to next track"
          title="Next"
          className="flex h-8 w-8 items-center justify-center rounded-lg text-text-secondary hover:bg-surface-raised hover:text-text disabled:opacity-40 disabled:hover:bg-transparent"
        >
          <SkipForward size={16} />
        </button>
      </div>
    </div>
  )
}
