import { useState } from 'react'
import { Link } from 'react-router-dom'
import { ImageIcon } from 'lucide-react'
import { Badge } from '../ui/Badge'
import type { LibraryEntry } from '../../types'

interface Props {
  entry: LibraryEntry
  href: string
  aspect?: '2/3' | '16/9' | '1/1'
  accent?: string
}

const KIND_LABELS: Record<string, string> = {
  network: 'Network',
  studio: 'Studio',
  series: 'Series',
  artist: 'Artist',
  movie: 'Movie',
  publisher: 'Publisher',
  book: 'Book',
}

export function EntryCard({ entry, href, aspect = '2/3', accent }: Props) {
  const [imgFailed, setImgFailed] = useState(false)
  const showImg = !!entry.imageUrl && !imgFailed

  return (
    <Link to={href} className="group flex flex-col gap-2 cursor-pointer">
      <div
        className="relative rounded-xl overflow-hidden bg-white/4 border border-white/5 group-hover:border-white/15 transition-all duration-200 group-hover:scale-[1.02]"
        style={{ aspectRatio: aspect }}
      >
        {showImg ? (
          <img
            src={entry.imageUrl}
            alt={entry.name}
            className="w-full h-full object-contain p-2"
            loading="lazy"
            onError={() => setImgFailed(true)}
          />
        ) : (
          <div className="w-full h-full flex items-center justify-center">
            <ImageIcon size={36} className="text-white/10" strokeWidth={1} />
          </div>
        )}
        {/* Hover ring */}
        <div
          className="absolute inset-0 opacity-0 group-hover:opacity-100 transition-opacity duration-200 pointer-events-none rounded-xl"
          style={{ boxShadow: accent ? `inset 0 0 0 1.5px ${accent}55` : undefined }}
        />
        {/* Bottom gradient */}
        <div className="absolute bottom-0 inset-x-0 h-20 bg-gradient-to-t from-black/70 to-transparent pointer-events-none" />
        {/* Status badge */}
        {entry.status && entry.status !== 'active' && (
          <div className="absolute top-2 right-2">
            <Badge color={entry.status === 'ended' ? '#ef4444' : undefined}>
              {entry.status}
            </Badge>
          </div>
        )}
      </div>

      <div className="px-0.5">
        <p className="text-sm font-medium text-white/90 truncate leading-tight">{entry.name}</p>
        <p className="text-xs text-white/35 truncate mt-0.5 capitalize">{KIND_LABELS[entry.kind] ?? entry.kind}</p>
      </div>
    </Link>
  )
}
