import { useState } from 'react'
import { Link } from 'react-router-dom'
import { ImageIcon } from 'lucide-react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { patchGroup } from '../api/groups'
import { Badge } from './ui/Badge'
import type { Group } from '../types'

export function albumMonitorLabel(monitored: boolean): string {
  return monitored ? 'Monitored — click to unmonitor' : 'Unmonitored — click to monitor'
}

interface AlbumCardProps {
  album: Group
  href: string
  showMonitorBadge?: boolean
  accent?: string
}

export function AlbumCard({ album, href, showMonitorBadge = false, accent = '#10b981' }: AlbumCardProps) {
  const queryClient = useQueryClient()
  const [imgFailed, setImgFailed] = useState(false)

  const toggleMonitor = useMutation({
    mutationFn: () => patchGroup(album.id, { monitored: !album.monitored }),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['groups'] }),
  })

  const showCover = !!album.coverUrl && !imgFailed

  return (
    <div className="group flex flex-col gap-2">
      <div className="relative rounded-xl overflow-hidden bg-white/4 border border-white/5 group-hover:border-white/15 transition-all duration-200 group-hover:scale-[1.02]" style={{ aspectRatio: '1/1' }}>
        <Link to={href} className="block w-full h-full flex items-center justify-center">
          {showCover ? (
            <img
              src={album.coverUrl}
              alt={album.title}
              className="w-full h-full object-cover"
              onError={() => setImgFailed(true)}
            />
          ) : (
            <ImageIcon size={32} className="text-white/10" strokeWidth={1} />
          )}
        </Link>

        <div
          className="absolute inset-0 opacity-0 group-hover:opacity-100 transition-opacity duration-200 pointer-events-none rounded-xl"
          style={{ boxShadow: `inset 0 0 0 1.5px ${accent}55` }}
        />
        <div className="absolute bottom-0 inset-x-0 h-16 bg-gradient-to-t from-black/70 to-transparent pointer-events-none" />

        {showMonitorBadge && (
          <button
            onClick={() => toggleMonitor.mutate()}
            className="absolute bottom-2 left-2 pointer-events-auto"
            title={albumMonitorLabel(album.monitored)}
          >
            <Badge color={album.monitored ? accent : undefined}>
              {album.monitored ? 'Monitored' : 'Unmonitored'}
            </Badge>
          </button>
        )}
      </div>

      <div className="px-0.5">
        <Link to={href} className="text-sm font-medium text-white/80 truncate hover:text-white block">
          {album.title}
        </Link>
        {album.year > 0 && <p className="text-xs text-white/35">{album.year}</p>}
      </div>
    </div>
  )
}
