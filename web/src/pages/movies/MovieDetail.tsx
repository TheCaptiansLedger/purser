import { useParams, Link } from 'react-router-dom'
import { ArrowLeft, Calendar, Clock, ImageIcon } from 'lucide-react'
import { useLibraryEntry } from '../../api/library'
import { useItems } from '../../api/items'
import { Hero } from '../../components/layout/Hero'
import { Badge } from '../../components/ui/Badge'
import { PersonCard } from '../../components/media/PersonCard'
import { fmtRuntime, fmtBytes } from '../../components/ui/Runtime'
import { Skeleton } from '../../components/ui/Skeleton'

const ACCENT = '#3b82f6'

export function MovieDetail() {
  const { id } = useParams<{ id: string }>()
  const { data: entry, isLoading } = useLibraryEntry(id!)
  const { data: itemsPage } = useItems({ libraryEntryId: id!, limit: 1 })
  const item = itemsPage?.data[0]

  if (isLoading) {
    return (
      <div className="px-8 py-10 space-y-6">
        <Skeleton className="h-64 w-full" />
        <div className="flex gap-6">
          <Skeleton className="w-48 h-72 shrink-0" />
          <div className="flex-1 space-y-3">
            <Skeleton className="h-8 w-2/3" />
            <Skeleton className="h-4 w-1/3" />
            <Skeleton className="h-20 w-full" />
          </div>
        </div>
      </div>
    )
  }

  if (!entry) return null

  const performers = item?.people.filter(p => p.role === 'actor' || p.role === 'actress') ?? []

  return (
    <div>
      {/* Back */}
      <div className="px-8 pt-6">
        <Link to="/movies" className="inline-flex items-center gap-1.5 text-sm text-white/40 hover:text-white/70 transition-colors">
          <ArrowLeft size={14} />
          Movies
        </Link>
      </div>

      {/* Hero */}
      <Hero backdropUrl={entry.imageUrl ?? item?.coverUrl} accent={ACCENT}>
        <div className="flex gap-6 items-end">
          {/* Poster */}
          <div className="shrink-0 w-44 rounded-xl overflow-hidden border border-white/10 shadow-2xl" style={{ aspectRatio: '2/3' }}>
            {entry.imageUrl ? (
              <img src={entry.imageUrl} alt={entry.name} className="w-full h-full object-cover" />
            ) : (
              <div className="w-full h-full bg-white/5 flex items-center justify-center">
                <ImageIcon size={40} className="text-white/15" strokeWidth={1} />
              </div>
            )}
          </div>

          {/* Metadata */}
          <div className="flex-1 min-w-0">
            <h1 className="text-3xl font-bold text-white mb-2 leading-tight">{entry.name}</h1>
            <div className="flex flex-wrap items-center gap-2 mb-3">
              {entry.status && <Badge color={ACCENT}>{entry.status}</Badge>}
              {item?.date && (
                <span className="flex items-center gap-1 text-sm text-white/50">
                  <Calendar size={13} />{new Date(item.date).getFullYear()}
                </span>
              )}
              {item?.runtimeSeconds ? (
                <span className="flex items-center gap-1 text-sm text-white/50">
                  <Clock size={13} />{fmtRuntime(item.runtimeSeconds)}
                </span>
              ) : null}
              {item?.mediaFile?.quality && <Badge>{item.mediaFile.quality}</Badge>}
            </div>
            {entry.overview && (
              <p className="text-sm text-white/60 leading-relaxed max-w-2xl line-clamp-4">{entry.overview}</p>
            )}
          </div>
        </div>
      </Hero>

      <div className="px-8 py-8 space-y-10">
        {/* Cast */}
        {performers.length > 0 && (
          <section>
            <h2 className="text-sm font-semibold text-white/40 uppercase tracking-widest mb-4">Cast</h2>
            <div className="grid grid-cols-3 sm:grid-cols-4 md:grid-cols-6 lg:grid-cols-8 gap-3">
              {performers.map(({ person, personId }) => person ? (
                <PersonCard key={personId} person={{ id: personId, name: person.name, sortName: person.sortName, imageUrl: person.imageUrl, overview: '', monitored: false, monitorMode: 'all', aliases: [], externalIds: [], addedAt: '' }} href={`/people/${personId}`} accent={ACCENT} />
              ) : null)}
            </div>
          </section>
        )}

        {/* Tags */}
        {entry.tags.length > 0 && (
          <section>
            <h2 className="text-sm font-semibold text-white/40 uppercase tracking-widest mb-3">Tags</h2>
            <div className="flex flex-wrap gap-2">
              {entry.tags.map(t => <Badge key={t.id}>{t.name}</Badge>)}
            </div>
          </section>
        )}

        {/* File info */}
        {item?.mediaFile && (
          <section>
            <h2 className="text-sm font-semibold text-white/40 uppercase tracking-widest mb-3">File</h2>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
              {[
                { label: 'Path', value: item.mediaFile.path },
                { label: 'Size', value: fmtBytes(item.mediaFile.size) },
                { label: 'Quality', value: item.mediaFile.quality },
                { label: 'Resolution', value: item.mediaFile.resolution },
                { label: 'Codec', value: item.mediaFile.codec },
                { label: 'Container', value: item.mediaFile.container },
              ].filter(r => r.value).map(({ label, value }) => (
                <div key={label} className="bg-white/3 rounded-lg p-3">
                  <p className="text-xs text-white/35 mb-0.5">{label}</p>
                  <p className="text-sm text-white/80 truncate">{value}</p>
                </div>
              ))}
            </div>
          </section>
        )}

        {/* External IDs */}
        {entry.externalIds.length > 0 && (
          <section>
            <h2 className="text-sm font-semibold text-white/40 uppercase tracking-widest mb-3">External IDs</h2>
            <div className="flex flex-wrap gap-2">
              {entry.externalIds.map(e => (
                <span key={e.source} className="text-xs bg-white/5 px-2 py-1 rounded-md text-white/50">
                  {e.source}: {e.value}
                </span>
              ))}
            </div>
          </section>
        )}
      </div>
    </div>
  )
}
