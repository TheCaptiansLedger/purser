import { useParams, Link } from 'react-router-dom'
import { ArrowLeft, ImageIcon, ChevronRight } from 'lucide-react'
import { useLibraryEntry } from '../../api/library'
import { useGroups } from '../../api/groups'
import { Hero } from '../../components/layout/Hero'
import { Badge } from '../../components/ui/Badge'
import { Skeleton } from '../../components/ui/Skeleton'

const ACCENT = '#8b5cf6'

export function SeriesDetail() {
  const { id } = useParams<{ id: string }>()
  const { data: entry, isLoading } = useLibraryEntry(id!)
  const { data: groupsPage } = useGroups(id!)

  if (isLoading) return <div className="px-8 py-10"><Skeleton className="h-64 w-full" /></div>
  if (!entry) return null

  const seasons = groupsPage?.data ?? []

  return (
    <div>
      <div className="px-8 pt-6">
        <Link to="/tv" className="inline-flex items-center gap-1.5 text-sm text-white/40 hover:text-white/70 transition-colors">
          <ArrowLeft size={14} /> TV Shows
        </Link>
      </div>

      <Hero backdropUrl={entry.imageUrl} accent={ACCENT}>
        <div className="flex gap-6 items-end">
          <div className="shrink-0 w-40 rounded-xl overflow-hidden border border-white/10 shadow-2xl" style={{ aspectRatio: '2/3' }}>
            {entry.imageUrl ? (
              <img src={entry.imageUrl} alt={entry.name} className="w-full h-full object-cover" />
            ) : (
              <div className="w-full h-full bg-white/5 flex items-center justify-center">
                <ImageIcon size={36} className="text-white/15" strokeWidth={1} />
              </div>
            )}
          </div>
          <div className="flex-1 min-w-0">
            <h1 className="text-3xl font-bold text-white mb-2">{entry.name}</h1>
            <div className="flex flex-wrap items-center gap-2 mb-3">
              <Badge color={entry.status === 'continuing' ? ACCENT : entry.status === 'ended' ? '#ef4444' : undefined}>
                {entry.status}
              </Badge>
              {seasons.length > 0 && <span className="text-sm text-white/40">{seasons.length} season{seasons.length !== 1 ? 's' : ''}</span>}
            </div>
            {entry.overview && (
              <p className="text-sm text-white/60 leading-relaxed max-w-2xl line-clamp-4">{entry.overview}</p>
            )}
          </div>
        </div>
      </Hero>

      <div className="px-8 py-8">
        <h2 className="text-sm font-semibold text-white/40 uppercase tracking-widest mb-4">Seasons</h2>
        {seasons.length === 0 ? (
          <p className="text-white/30 text-sm">No seasons added yet.</p>
        ) : (
          <div className="space-y-2">
            {seasons.map(season => (
              <Link
                key={season.id}
                to={`/tv/${id}/seasons/${season.number}`}
                className="flex items-center gap-4 p-4 rounded-xl bg-white/3 border border-white/5 hover:bg-white/6 hover:border-white/12 transition-all duration-150 group"
              >
                <div
                  className="w-10 h-10 rounded-lg flex items-center justify-center shrink-0 font-semibold text-sm"
                  style={{ background: ACCENT + '22', color: ACCENT }}
                >
                  {season.number}
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium text-white/80 group-hover:text-white transition-colors">
                    {season.title || `Season ${season.number}`}
                  </p>
                  {season.year > 0 && <p className="text-xs text-white/35">{season.year}</p>}
                </div>
                <ChevronRight size={16} className="text-white/20 group-hover:text-white/50 transition-colors shrink-0" />
              </Link>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
