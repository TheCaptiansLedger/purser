import { useParams, Link } from 'react-router-dom'
import { ArrowLeft, Music2, Eye, EyeOff, SkipForward, BookmarkCheck } from 'lucide-react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useLibraryEntry } from '../../api/library'
import { useGroup, patchGroup } from '../../api/groups'
import { useItems, patchItem } from '../../api/items'
import { fmtRuntime } from '../../components/ui/Runtime'
import { Skeleton } from '../../components/ui/Skeleton'
import type { Item, ItemStatus } from '../../types'

const ACCENT = '#10b981'

const STATUS_DOT: Record<ItemStatus, string> = {
  wanted:      '#f59e0b',
  grabbed:     '#3b82f6',
  downloading: '#3b82f6',
  imported:    '#10b981',
  missing:     '#6b7280',
  skipped:     '#374151',
}

function TrackRow({ track }: { track: Item }) {
  const queryClient = useQueryClient()

  const toggleMonitor = useMutation({
    mutationFn: () => patchItem(track.id, { monitored: !track.monitored }),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['items'] }),
  })

  const setStatus = useMutation({
    mutationFn: (status: ItemStatus) => patchItem(track.id, { status }),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['items'] }),
  })

  const canSetWanted  = track.status === 'skipped' || track.status === 'missing'
  const canSetSkipped = track.status === 'wanted'

  return (
    <div className="flex items-center gap-4 px-3 py-2.5 rounded-lg hover:bg-white/4 transition-colors group">
      <span className="w-6 text-right text-xs text-white/25 font-mono shrink-0">
        {track.sequence || '—'}
      </span>

      <span
        className="w-2 h-2 rounded-full shrink-0"
        style={{ background: STATUS_DOT[track.status] ?? '#6b7280' }}
        title={track.status}
      />

      <span className="flex-1 text-sm text-white/75 group-hover:text-white/90 transition-colors truncate">
        {track.title}
      </span>

      {track.runtimeSeconds > 0 && (
        <span className="text-xs text-white/30 shrink-0">{fmtRuntime(track.runtimeSeconds)}</span>
      )}

      <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity shrink-0">
        <button
          onClick={() => toggleMonitor.mutate()}
          title={track.monitored ? 'Unmonitor' : 'Monitor'}
          className="p-1 rounded hover:bg-white/8 transition-colors"
          style={{ color: track.monitored ? ACCENT : 'rgba(255,255,255,0.25)' }}
        >
          {track.monitored ? <Eye size={13} /> : <EyeOff size={13} />}
        </button>
        {canSetWanted && (
          <button
            onClick={() => setStatus.mutate('wanted')}
            title="Mark wanted"
            className="p-1 rounded hover:bg-white/8 transition-colors text-white/30 hover:text-amber-400"
          >
            <BookmarkCheck size={13} />
          </button>
        )}
        {canSetSkipped && (
          <button
            onClick={() => setStatus.mutate('skipped')}
            title="Skip"
            className="p-1 rounded hover:bg-white/8 transition-colors text-white/30 hover:text-white/60"
          >
            <SkipForward size={13} />
          </button>
        )}
      </div>
    </div>
  )
}

export function AlbumDetail() {
  const { id, albumId } = useParams<{ id: string; albumId: string }>()
  const queryClient = useQueryClient()
  const { data: artist } = useLibraryEntry(id!)
  const { data: album, isLoading } = useGroup(albumId!)
  const { data: tracksPage } = useItems({ groupId: albumId!, limit: 500 })
  const tracks = tracksPage?.data ?? []

  const toggleAlbumMonitor = useMutation({
    mutationFn: () => patchGroup(albumId!, { monitored: !album?.monitored }),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['groups'] }),
  })

  if (isLoading) return <div className="px-8 py-10"><Skeleton className="h-48 w-full" /></div>
  if (!album) return null

  const totalRuntime = tracks.reduce((s, t) => s + t.runtimeSeconds, 0)
  const wantedCount  = tracks.filter(t => t.status === 'wanted').length
  const importedCount = tracks.filter(t => t.status === 'imported').length

  return (
    <div className="px-8 py-6">
      <div className="mb-6">
        <Link to={`/music/${id}`} className="inline-flex items-center gap-1.5 text-sm text-white/40 hover:text-white/70 transition-colors">
          <ArrowLeft size={14} /> {artist?.name ?? 'Artist'}
        </Link>
      </div>

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
            {importedCount > 0 && (
              <span style={{ color: ACCENT }}>{importedCount} imported</span>
            )}
            {wantedCount > 0 && (
              <span className="text-amber-400/70">{wantedCount} wanted</span>
            )}
          </div>
          <button
            onClick={() => toggleAlbumMonitor.mutate()}
            className="mt-3 flex items-center gap-1.5 text-xs transition-colors"
            style={{ color: album.monitored ? ACCENT : 'rgba(255,255,255,0.25)' }}
          >
            {album.monitored ? <Eye size={12} /> : <EyeOff size={12} />}
            {album.monitored ? 'Monitored' : 'Unmonitored'}
          </button>
        </div>
      </div>

      <div className="space-y-0.5">
        {tracks.map(track => (
          <TrackRow key={track.id} track={track} />
        ))}
      </div>
    </div>
  )
}
