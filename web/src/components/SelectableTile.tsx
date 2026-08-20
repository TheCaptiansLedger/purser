import { Check } from 'lucide-react'
import type { ReactNode } from 'react'
import { Link } from 'react-router-dom'

export interface SelectableTileProps {
  selectMode: boolean
  selected: boolean
  onToggle: () => void
  children: ReactNode
  // Plain <Link> navigation (ArtistCard/AlbumCard: no nested interactive
  // element of their own) — mutually exclusive with onNavigate/ariaLabel
  // below; exactly one of the two modes applies per call site.
  to?: string
  // Click-target-detection navigation (PersonCard: its photo is itself a
  // button, so the tile can't be a <Link> — see People.tsx's own
  // pre-existing precedent this mirrors) — a click landing on a nested
  // button is left alone rather than triggering navigation.
  onNavigate?: () => void
  ariaLabel?: string
}

// SelectableTile — the bulk-select grid tile (#679, extended for People's
// bulk delete) shared across the Music Library grid (ArtistCard), the
// Discography tab (AlbumCard), and the People index page (PersonCard).
// Card/ArtistCard/AlbumCard/PersonCard stay untouched: same "caller wraps
// for navigation" precedent Card's own doc comment sets — this just adds
// a third wrap mode (toggle-button + checkbox overlay) alongside the two
// navigation ones already in use, chosen by the caller's selectMode flag,
// rather than teaching any Card variant about selection.
//
// Not in select mode: identical to whichever plain navigation wrap this
// replaced (`to` for a <Link>, `onNavigate`/`ariaLabel` for the
// click-target-detection div PersonCard's nested photo button needs). In
// select mode: a toggle `<button>` (no navigation) with a checkbox
// overlay and a selected-state ring, so entering select mode never fires
// a stray navigation from a click meant to select.
export function SelectableTile({ selectMode, selected, onToggle, children, to, onNavigate, ariaLabel }: SelectableTileProps) {
  if (!selectMode) {
    if (to !== undefined) {
      return (
        <Link to={to} className="rounded-lg hover:bg-surface-raised">
          {children}
        </Link>
      )
    }
    return (
      <div
        role="link"
        tabIndex={0}
        aria-label={ariaLabel}
        onClick={e => {
          if ((e.target as HTMLElement).closest('button')) return
          onNavigate?.()
        }}
        onKeyDown={e => {
          if (e.key === 'Enter') onNavigate?.()
        }}
        className="cursor-pointer rounded-lg hover:bg-surface-raised"
      >
        {children}
      </div>
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
