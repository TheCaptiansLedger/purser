import { useContext } from 'react'
import { PlayerContext, type PlayerContextValue } from '../components/PlayerProvider'

// usePlayer is the one way app code reads/drives player state — throws
// rather than returning a disconnected default when called outside
// PlayerProvider, so a future page that forgets it's not under Layout
// fails immediately instead of silently doing nothing.
export function usePlayer(): PlayerContextValue {
  const ctx = useContext(PlayerContext)
  if (!ctx) {
    throw new Error('usePlayer must be used within a PlayerProvider')
  }
  return ctx
}
