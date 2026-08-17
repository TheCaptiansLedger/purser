import { Disc3 } from 'lucide-react'
import type { GroupRef } from '../types'
import { Card } from './Card'
import { MusicReleaseStatusBadge } from './MusicReleaseStatusBadge'

export interface AlbumCardProps {
  album: GroupRef
}

// AlbumCard — the Artist Detail Discography tab's (#667) Card config
// (#658). title=Group.Title, subtitle=year, badge=MusicReleaseStatusBadge
// (#659) driven by the default edition's derived status (see
// useDiscography). Same no-click-behavior precedent as
// ArtistCard/PersonCard/Card — this component only renders; the caller
// wraps it for navigation once Album Detail (#673) exists.
//
// A Group with zero MusicRelease rows (album.status undefined) renders a
// defined "No edition selected" badge rather than a missing/blank one —
// #667's own acceptance criterion, kept distinct from
// MusicReleaseStatusBadge (#659 scopes that component to the 3 real
// ReleaseStatus values, not a 4th "no edition" case).
export function AlbumCard({ album }: AlbumCardProps) {
  const imageSrc = album.imageId ? `/media/images/${album.imageId}` : undefined

  return (
    <Card
      imageSrc={imageSrc}
      title={album.title}
      subtitle={album.year ? String(album.year) : undefined}
      placeholderIcon={Disc3}
      badge={
        album.status ? (
          <MusicReleaseStatusBadge status={album.status} />
        ) : (
          <span className="inline-flex items-center text-label font-medium text-status-neutral">
            No edition selected
          </span>
        )
      }
    />
  )
}
