import { Music } from 'lucide-react'
import type { LibraryEntryRef } from '../types'
import { Card } from './Card'

export interface ArtistCardProps {
  artist: LibraryEntryRef
}

// ArtistCard — the Music Library grid's (#664) Card config (#658).
// title=name, subtitle=genre (Card's own doc names this exact slot),
// badge=a plain monitored/not-monitored dot. Deliberately no ownership
// ring here — that's #670, a separate story. Same no-click-behavior
// precedent as PersonCard/Card: this component only renders, the Music
// Library page wraps it for navigation once Artist Detail (#671) exists.
export function ArtistCard({ artist }: ArtistCardProps) {
  const imageSrc = artist.imageId ? `/media/images/${artist.imageId}` : undefined

  return (
    <Card
      imageSrc={imageSrc}
      title={artist.name}
      subtitle={artist.genre}
      placeholderIcon={Music}
      badge={
        <span
          role="img"
          aria-label={artist.monitored ? 'Monitored' : 'Not monitored'}
          title={artist.monitored ? 'Monitored' : 'Not monitored'}
          className={[
            'block h-2.5 w-2.5 rounded-full border border-bg',
            artist.monitored ? 'bg-status-success' : 'bg-status-neutral',
          ].join(' ')}
        />
      }
    />
  )
}
