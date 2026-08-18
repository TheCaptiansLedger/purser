import { Music } from 'lucide-react'
import type { LibraryEntryRef } from '../types'
import { Card } from './Card'
import { OwnershipRing, type OwnershipRingProps } from './OwnershipRing'

export interface ArtistCardProps {
  artist: LibraryEntryRef
  // Fan-out result from useLibraryOwnership (#670), kept as a separate
  // prop rather than folded into LibraryEntryRef — it's page-level
  // fan-out data, not part of the artist's own identity. Optional so
  // ArtistCard still renders standalone (e.g. in a test) without a
  // caller that's done the fan-out.
  ownership?: OwnershipRingProps
}

// ArtistCard — the Music Library grid's (#664) Card config (#658).
// title=name, subtitle=genre (Card's own doc names this exact slot),
// badge=the monitored/not-monitored dot plus the ownership ring (#670)
// stacked together in Card's single corner-badge slot — Card itself
// stays unchanged, it never branches on what's inside that slot. Same
// no-click-behavior precedent as PersonCard/Card: this component only
// renders, the Music Library page wraps it for navigation once Artist
// Detail (#671) exists.
export function ArtistCard({ artist, ownership }: ArtistCardProps) {
  const imageSrc = artist.imageId ? `/media/images/${artist.imageId}` : undefined

  return (
    <Card
      imageSrc={imageSrc}
      title={artist.name}
      subtitle={artist.genre}
      placeholderIcon={Music}
      badge={
        <div className="flex items-center gap-1">
          {ownership && <OwnershipRing {...ownership} />}
          <span
            role="img"
            aria-label={artist.monitored ? 'Monitored' : 'Not monitored'}
            title={artist.monitored ? 'Monitored' : 'Not monitored'}
            className={[
              'block h-2.5 w-2.5 rounded-full border border-bg',
              artist.monitored ? 'bg-status-success' : 'bg-status-neutral',
            ].join(' ')}
          />
        </div>
      }
    />
  )
}
