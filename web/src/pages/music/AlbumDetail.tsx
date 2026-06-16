import { useParams, Link } from 'react-router-dom'
import { ArrowLeft, Music2 } from 'lucide-react'
import { useLibraryEntry } from '../../api/library'
import { useGroup } from '../../api/groups'
import { useItems } from '../../api/items'
import { fmtRuntime } from '../../components/ui/Runtime'
import { Skeleton } from '../../components/ui/Skeleton'

const ACCENT = '#10b981'

export function AlbumDetail() {
  const { id, albumId } = useParams<{ id: string; albumId: string }>()
  const { data: artist } = useLibraryEntry(id!)
  const { data: album, isLoading } = useGroup(albumId!)
  const { data: tracksPage } = useItems({ groupId: albumId!, limit: 500 })
  const tracks = tracksPage?.data ?? []

  if (isLoading) return <div className="px-8 py-10"><Skeleton className="h-48 w-full" /></div>
  if (!album) return null

  const totalRuntime = tracks.reduce((s, t) => s + t.runtimeSeconds, 0)

  return (
    <div className="px-8 py-6">
      <div className="mb-6">
        <Link to={`/music/${id}`} className="inline-flex items-center gap-1.5 text-sm text-white/40 hover:text-white/70 transition-colors">
          <ArrowLeft size={14} /> {artist?.name ?? 'Artist'}
        </Link>
      </div>

      {/* Album header */}
      <div className="flex gap-6 items-start mb-8">
        <div className="shrink-0 w-40 h-40 rounded-xl overflow-hidden bg-white/5 border border-white/8 flex items-center justify-center shadow-2xl">
          <Music2 size={48} className="text-white/10" strokeWidth={1} />
        </div>
        <div>
          <p className="text-xs font-medium uppercase tracking-widest mb-1" style={{ color: ACCENT }}>Album</p>
          <h1 className="text-2xl font-bold text-white mb-1">{album.title}</h1>
          {artist && <p className="text-white/50 text-sm">{artist.name}</p>}
          <div className="flex items-center gap-3 mt-2 text-xs text-white/35">
            {album.year > 0 && <span>{album.year}</span>}
            {tracks.length > 0 && <span>{tracks.length} track{tracks.length !== 1 ? 's' : ''}</span>}
            {totalRuntime > 0 && <span>{fmtRuntime(totalRuntime)}</span>}
          </div>
        </div>
      </div>

      {/* Tracklist */}
      <div className="space-y-0.5">
        {tracks.map((track, i) => (
          <div
            key={track.id}
            className="flex items-center gap-4 px-3 py-2.5 rounded-lg hover:bg-white/4 transition-colors group"
          >
            <span className="w-6 text-right text-xs text-white/25 font-mono shrink-0">{track.sequence || i + 1}</span>
            <span className="flex-1 text-sm text-white/75 group-hover:text-white/90 transition-colors truncate">{track.title}</span>
            {track.runtimeSeconds > 0 && (
              <span className="flex items-center gap-1 text-xs text-white/30 shrink-0">
                {fmtRuntime(track.runtimeSeconds)}
              </span>
            )}
          </div>
        ))}
      </div>
    </div>
  )
}
