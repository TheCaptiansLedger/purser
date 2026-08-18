import type { ReactNode } from 'react'

export interface OwnershipRingProps {
  // Count of albums whose default edition is currently Imported ("owned").
  owned: number
  // Total albums (Groups) the artist has at all — the ring's denominator.
  total: number
  isPending?: boolean
  isError?: boolean
}

const SIZE = 16
const STROKE = 2.5
const RADIUS = (SIZE - STROKE) / 2
const CIRCUMFERENCE = 2 * Math.PI * RADIUS

// OwnershipRing — style-guide Component vocabulary entry
// (docs/technical/music-web-ui.md), generic "N of M owned" fractional-
// progress ring, not Music-specific — the Music Library grid (#670) is
// its first caller, feeding it useLibraryOwnership's per-artist result.
//
// Three states, each a defined visual rather than a blank/missing one:
// - isPending: a skeleton pulse, no fraction shown yet (fan-out in
//   flight — never a full-page spinner, per #670's own acceptance
//   criterion).
// - isError: a muted, dashed ring — this card's fan-out failed, but that
//   never means the ring silently disappears (matches AlbumCard's own
//   "never blank" precedent for a Group's status).
// - total === 0 renders nothing: an artist with no albums yet has
//   nothing to show a fraction of, unlike AlbumCard's "no edition
//   selected" case (that's an existing album missing data; this is
//   simply zero albums, the common state for a freshly added artist).
// Wrapper — the pill both the pending/error/normal states render into, so
// the ring reads clearly over any artwork color underneath it.
function Pill({ children, label }: { children: ReactNode; label: string }) {
  return (
    <div
      role="img"
      aria-label={label}
      title={label}
      className="flex h-5 items-center gap-1 rounded-full bg-surface/90 px-1.5"
    >
      {children}
    </div>
  )
}

export function OwnershipRing({ owned, total, isPending, isError }: OwnershipRingProps) {
  if (isPending) {
    return (
      <div
        role="img"
        aria-label="Loading album ownership"
        className="h-5 w-9 animate-pulse rounded-full bg-surface-raised"
      />
    )
  }

  if (isError) {
    return (
      <Pill label="Couldn't load album ownership">
        <svg width={SIZE} height={SIZE} aria-hidden="true">
          <circle
            cx={SIZE / 2}
            cy={SIZE / 2}
            r={RADIUS}
            fill="none"
            strokeDasharray="1.5 2"
            className="stroke-status-neutral"
            strokeWidth={STROKE}
          />
        </svg>
        <span className="text-label font-medium text-status-neutral">–</span>
      </Pill>
    )
  }

  if (total === 0) {
    return null
  }

  const fraction = Math.min(owned / total, 1)
  const offset = CIRCUMFERENCE * (1 - fraction)

  return (
    <Pill label={`${owned} of ${total} albums owned`}>
      <svg width={SIZE} height={SIZE} className="-rotate-90" aria-hidden="true">
        <circle
          cx={SIZE / 2}
          cy={SIZE / 2}
          r={RADIUS}
          fill="none"
          strokeWidth={STROKE}
          className="stroke-border"
        />
        <circle
          cx={SIZE / 2}
          cy={SIZE / 2}
          r={RADIUS}
          fill="none"
          strokeWidth={STROKE}
          strokeLinecap="round"
          strokeDasharray={CIRCUMFERENCE}
          strokeDashoffset={offset}
          className="stroke-status-success"
        />
      </svg>
      <span className="text-label font-medium text-text">
        {owned}/{total}
      </span>
    </Pill>
  )
}
