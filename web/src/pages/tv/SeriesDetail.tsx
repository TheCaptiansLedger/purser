import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { ArrowLeft, ImageIcon, ChevronRight } from 'lucide-react'
import { useLibraryEntry } from '../../api/library'
import { useGroups } from '../../api/groups'
import { useImageVersion } from '../../hooks/useImageVersion'
import { LibraryEntryEditor } from '../../components/edit/editors/LibraryEntryEditor'
import { Hero } from '../../components/layout/Hero'
import { Badge } from '../../components/ui/Badge'
import { Skeleton } from '../../components/ui/Skeleton'
import { filterTagsForModule } from '../../utils/filterTagsForModule'

const ACCENT = '#8b5cf6'

export function SeriesDetail() {
  const { id } = useParams<{ id: string }>()
  const [editOpen, setEditOpen] = useState(false)

  const { data: entry, isLoading } = useLibraryEntry(id!)
  const { data: groupsPage } = useGroups(id!)
  const [versionedImageUrl, bumpImageVersion] = useImageVersion(entry?.imageUrl)

  if (isLoading) return <div className="px-8 py-10"><Skeleton className="h-64 w-full" /></div>
  if (!entry) return null

  const seasons = groupsPage?.data ?? []
  const visibleTags = filterTagsForModule(entry.tags, 'tv')
  const networkTags = visibleTags.filter(t => t.key === 'network')
  const otherTags = visibleTags.filter(t => t.key !== 'network')

  return (
    <div>
      <div className="px-8 pt-6 flex items-center justify-between">
        <Link to="/tv" className="inline-flex items-center gap-1.5 text-sm text-white/40 hover:text-white/70 transition-colors">
          <ArrowLeft size={14} /> TV Shows
        </Link>
        <button
          onClick={() => setEditOpen(true)}
          className="inline-flex items-center gap-1.5 text-xs font-medium px-3 py-1.5 rounded-lg border border-white/10 text-white/50 hover:text-white/80 hover:border-white/20 transition-colors"
        >
          Edit
        </button>
      </div>

      <Hero backdropUrl={entry.imageUrl} accent={ACCENT}>
        <div className="flex gap-6 items-end">
          <div className="shrink-0 w-40 rounded-xl overflow-hidden border border-white/10 shadow-2xl" style={{ aspectRatio: '2/3' }}>
            {entry.imageUrl ? (
              <img src={versionedImageUrl} alt={entry.name} className="w-full h-full object-cover" />
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

      <div className="px-8 py-8 space-y-8">
        {(networkTags.length > 0 || otherTags.length > 0) && (
          <div className="space-y-4">
            {networkTags.length > 0 && (
              <div>
                <h2 className="text-sm font-semibold text-white/40 uppercase tracking-widest mb-3">Network</h2>
                <div className="flex flex-wrap gap-1.5">
                  {networkTags.map(t => (
                    <Link key={t.id} to={`/tags/${encodeURIComponent(t.key)}/${encodeURIComponent(t.value)}`} className="inline-flex items-center rounded-full px-2.5 py-1 text-xs font-medium border hover:opacity-80 transition-opacity" style={{ borderColor: ACCENT + '44', color: ACCENT }}>
                      {t.value}
                    </Link>
                  ))}
                </div>
              </div>
            )}
            {otherTags.length > 0 && (
              <div>
                <h2 className="text-sm font-semibold text-white/40 uppercase tracking-widest mb-3">Tags</h2>
                <div className="flex flex-wrap gap-1.5">
                  {otherTags.map(t => (
                    <Link key={t.id} to={`/tags/${encodeURIComponent(t.key)}/${encodeURIComponent(t.value)}`} className="inline-flex items-center rounded-full px-2.5 py-1 text-xs font-medium border border-white/10 text-white/50 hover:text-white/80 hover:border-white/20 transition-colors">
                      {t.value}
                    </Link>
                  ))}
                </div>
              </div>
            )}
          </div>
        )}

        <div>
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

      {editOpen && (
        <LibraryEntryEditor
          entry={entry}
          onClose={() => setEditOpen(false)}
          onImageSet={bumpImageVersion}
        />
      )}
    </div>
  )
}
