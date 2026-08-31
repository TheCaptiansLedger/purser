// formatRuntime renders a duration in seconds as "m:ss" — extracted out of
// Tracklist.tsx (originally private there) so NowPlayingBar (#750) can
// share the exact same formatting instead of growing its own copy.
export function formatRuntime(seconds: number): string | undefined {
  if (seconds <= 0) return undefined
  const m = Math.floor(seconds / 60)
  const s = Math.floor(seconds % 60)
  return `${m}:${String(s).padStart(2, '0')}`
}
