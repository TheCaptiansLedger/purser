import { useParams, Link } from 'react-router-dom'
import { ArrowLeft, Clock, ImageIcon } from 'lucide-react'
import { useLibraryEntry } from '../../api/library'
import { useGroups } from '../../api/groups'
import { useItems } from '../../api/items'
import { fmtRuntime } from '../../components/ui/Runtime'
import { Skeleton } from '../../components/ui/Skeleton'

const ACCENT = '#8b5cf6'

export function SeasonDetail() {
  const { id, num } = useParams<{ id: string; num: string }>()
  const { data: entry } = useLibraryEntry(id!)
  const { data: groupsPage } = useGroups(id!)

  const season = groupsPage?.data.find(g => g.number === Number(num))
  const { data: itemsPage, isLoading } = useItems({
    groupId: season?.id,
    libraryEntryId: id!,
    limit: 200,
  })

  if (!season && groupsPage) return (
    <div className="px-8 py-10 text-white/40">Season not found.</div>
  )

  const episodes = itemsPage?.data ?? []

  return (
    <div className="px-8 py-6">
      <div className="mb-6">
        <Link to={`/tv/${id}`} className="inline-flex items-center gap-1.5 text-sm text-white/40 hover:text-white/70 transition-colors">
          <ArrowLeft size={14} />
          {entry?.name ?? 'Series'}
        </Link>
      </div>

      <div className="mb-8">
        <h1 className="text-2xl font-bold text-white">{season?.title || `Season ${num}`}</h1>
        {season?.year ? <p className="text-white/40 text-sm mt-1">{season.year}</p> : null}
        {season?.overview && <p className="text-white/55 text-sm mt-2 max-w-2xl">{season.overview}</p>}
      </div>

      {isLoading ? (
        <div className="space-y-2">{Array.from({ length: 8 }).map((_, i) => <Skeleton key={i} className="h-20 w-full" />)}</div>
      ) : (
        <div className="space-y-1.5">
          {episodes.map(ep => (
            <div
              key={ep.id}
              className="flex items-center gap-4 p-3.5 rounded-xl bg-white/3 border border-white/5 hover:bg-white/5 hover:border-white/10 transition-all duration-150 group"
            >
              {/* Thumbnail */}
              <div className="shrink-0 w-28 rounded-lg overflow-hidden bg-white/5" style={{ aspectRatio: '16/9' }}>
                {ep.coverUrl ? (
                  <img src={ep.coverUrl} alt={ep.title} className="w-full h-full object-cover" />
                ) : (
                  <div className="w-full h-full flex items-center justify-center">
                    <ImageIcon size={18} className="text-white/15" strokeWidth={1} />
                  </div>
                )}
              </div>

              {/* Info */}
              <div className="flex-1 min-w-0">
                <div className="flex items-baseline gap-2">
                  {ep.sequence && (
                    <span className="text-xs font-mono" style={{ color: ACCENT }}>{ep.sequence}</span>
                  )}
                  <span className="text-sm font-medium text-white/80 truncate">{ep.title}</span>
                </div>
                {ep.date && <p className="text-xs text-white/30 mt-0.5">{new Date(ep.date).toLocaleDateString()}</p>}
                {ep.overview && <p className="text-xs text-white/40 mt-1 line-clamp-2">{ep.overview}</p>}
              </div>

              {/* Runtime */}
              {ep.runtimeSeconds > 0 && (
                <span className="shrink-0 flex items-center gap-1 text-xs text-white/30">
                  <Clock size={11} />{fmtRuntime(ep.runtimeSeconds)}
                </span>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
