import { useParams, Link } from 'react-router-dom'
import { ArrowLeft, ImageIcon } from 'lucide-react'
import { useLibraryEntry } from '../../api/library'
import { useGroups } from '../../api/groups'
import { Hero } from '../../components/layout/Hero'
import { PersonCard } from '../../components/media/PersonCard'
import { Skeleton } from '../../components/ui/Skeleton'

const ACCENT = '#10b981'

export function ArtistDetail() {
  const { id } = useParams<{ id: string }>()
  const { data: entry, isLoading } = useLibraryEntry(id!)
  const { data: albumsPage } = useGroups(id!)
  const albums = albumsPage?.data ?? []

  if (isLoading) return <div className="px-8 py-10"><Skeleton className="h-64 w-full" /></div>
  if (!entry) return null

  return (
    <div>
      <div className="px-8 pt-6">
        <Link to="/music" className="inline-flex items-center gap-1.5 text-sm text-white/40 hover:text-white/70 transition-colors">
          <ArrowLeft size={14} /> Music
        </Link>
      </div>

      {/* Full-width artist hero */}
      <Hero backdropUrl={entry.imageUrl} accent={ACCENT}>
        <div className="flex gap-6 items-end">
          <div className="shrink-0 w-36 h-36 rounded-full overflow-hidden border-2 shadow-2xl" style={{ borderColor: ACCENT + '44' }}>
            {entry.imageUrl ? (
              <img src={entry.imageUrl} alt={entry.name} className="w-full h-full object-cover" />
            ) : (
              <div className="w-full h-full bg-white/5 flex items-center justify-center">
                <ImageIcon size={32} className="text-white/15" strokeWidth={1} />
              </div>
            )}
          </div>
          <div>
            <p className="text-xs font-medium uppercase tracking-widest mb-1" style={{ color: ACCENT }}>Artist</p>
            <h1 className="text-4xl font-bold text-white mb-2">{entry.name}</h1>
            {albums.length > 0 && (
              <p className="text-sm text-white/40">{albums.length} album{albums.length !== 1 ? 's' : ''}</p>
            )}
          </div>
        </div>
      </Hero>

      <div className="px-8 py-8 space-y-8">
        {entry.overview && (
          <p className="text-sm text-white/60 leading-relaxed max-w-3xl">{entry.overview}</p>
        )}

        {entry.people.length > 0 && (
          <section>
            <h2 className="text-sm font-semibold text-white/40 uppercase tracking-widest mb-4">Members</h2>
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-4">
              {entry.people.map(ep => ep.person && (
                <PersonCard
                  key={ep.personId}
                  person={{ ...ep.person, aliases: [], monitored: false, monitorMode: 'all', overview: '', externalIds: [], addedAt: '' }}
                  href={`/people/${ep.personId}`}
                  accent={ACCENT}
                />
              ))}
            </div>
          </section>
        )}

        <section>
          <h2 className="text-sm font-semibold text-white/40 uppercase tracking-widest mb-4">Discography</h2>
          {albums.length === 0 ? (
            <p className="text-white/30 text-sm">No albums added yet.</p>
          ) : (
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-4">
              {albums.map(album => (
                <Link
                  key={album.id}
                  to={`/music/${id}/albums/${album.id}`}
                  className="group flex flex-col gap-2"
                >
                  <div className="rounded-xl overflow-hidden bg-white/4 border border-white/5 group-hover:border-white/15 transition-all duration-200 group-hover:scale-[1.02]" style={{ aspectRatio: '1/1' }}>
                    <div className="w-full h-full flex items-center justify-center">
                      <ImageIcon size={32} className="text-white/10" strokeWidth={1} />
                    </div>
                  </div>
                  <div className="px-0.5">
                    <p className="text-sm font-medium text-white/80 truncate">{album.title}</p>
                    {album.year > 0 && <p className="text-xs text-white/35">{album.year}</p>}
                  </div>
                </Link>
              ))}
            </div>
          )}
        </section>
      </div>
    </div>
  )
}
