import { Check } from 'lucide-react'
import type { ReactNode } from 'react'
import { Link } from 'react-router-dom'

export interface SelectableTileProps {
  to: string
  selectMode: boolean
  selected: boolean
  onToggle: () => void
  children: ReactNode
}

// SelectableTile — the bulk-select grid tile (#679) shared between the
// Music Library grid (ArtistCard) and the Discography tab (AlbumCard),
// per ADR 0004's shared-component rule. Card/ArtistCard/AlbumCard stay
// untouched: same "caller wraps for navigation" precedent Card's own doc
// comment sets (ArtistCard's Link wrap in MusicLibrary, AlbumCard's in
// ArtistDetail) — this just adds a second wrap mode alongside the
// existing Link one, chosen by the caller's selectMode flag, rather than
// teaching Card itself about selection.
//
// Not in select mode: identical to the plain `<Link>` wrap this replaced.
// In select mode: a toggle `<button>` (no navigation) with a checkbox
// overlay and a selected-state ring, so entering select mode never fires
// a stray navigation from a click meant to select.
export function SelectableTile({ to, selectMode, selected, onToggle, children }: SelectableTileProps) {
  if (!selectMode) {
    return (
      <Link to={to} className="rounded-lg hover:bg-surface-raised">
        {children}
      </Link>
    )
  }

  return (
    <button
      type="button"
      onClick={onToggle}
      aria-pressed={selected}
      className="relative w-full rounded-lg text-left hover:bg-surface-raised"
    >
      <span
        aria-hidden="true"
        className={[
          'absolute left-2 top-2 z-10 flex h-5 w-5 items-center justify-center rounded-full border',
          selected ? 'border-accent-system bg-accent-system text-bg' : 'border-border bg-surface',
        ].join(' ')}
      >
        {selected && <Check size={14} />}
      </span>
      <div className={selected ? 'rounded-lg ring-2 ring-accent-system' : ''}>{children}</div>
    </button>
  )
}
