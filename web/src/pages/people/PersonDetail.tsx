import { useParams, Link } from 'react-router-dom'
import { ArrowLeft, User } from 'lucide-react'
import { usePerson } from '../../api/people'
import { useItems } from '../../api/items'
import { useLibraryEntries } from '../../api/library'
import { Badge } from '../../components/ui/Badge'
import { EntryCard } from '../../components/media/EntryCard'
import { ItemCard } from '../../components/media/ItemCard'
import { Skeleton } from '../../components/ui/Skeleton'

const ACCENT = '#6366f1'

export function PersonDetail() {
  const { id } = useParams<{ id: string }>()
  const { data: person, isLoading } = usePerson(id!)
  const { data: itemsPage } = useItems({ personId: id!, limit: 48 })
  const items = itemsPage?.data ?? []
  const { data: bandsPage } = useLibraryEntries({ personId: id! })
  const bands = bandsPage?.data ?? []

  if (isLoading) return <div className="px-8 py-10"><Skeleton className="h-64 w-full" /></div>
  if (!person) return null

  return (
    <div className="flex min-h-screen">
      {/* Left panel */}
      <aside className="w-72 shrink-0 sticky top-0 h-screen overflow-y-auto flex flex-col border-r border-white/5">
        <div className="relative" style={{ aspectRatio: '2/3' }}>
          {person.imageUrl ? (
            <img src={person.imageUrl} alt={person.name} className="w-full h-full object-cover object-top" />
          ) : (
            <div className="w-full h-full bg-white/3 flex items-center justify-center">
              <User size={64} className="text-white/10" strokeWidth={1} />
            </div>
          )}
          <div className="absolute inset-0 bg-gradient-to-t from-[#08080e] via-transparent to-transparent" />
        </div>

        <div className="px-5 pb-6 -mt-6 relative z-10">
          {person.aliases.length > 0 && (
            <div className="mb-4">
              <p className="text-xs text-white/30 uppercase tracking-wider mb-1.5">Also known as</p>
              <div className="flex flex-wrap gap-1.5">
                {person.aliases.map(a => (
                  <span key={a} className="text-xs text-white/50 bg-white/5 px-2 py-0.5 rounded-md">{a}</span>
                ))}
              </div>
            </div>
          )}

          {person.externalIds.length > 0 && (
            <div className="pt-3 border-t border-white/5">
              <p className="text-xs text-white/30 uppercase tracking-wider mb-2">External IDs</p>
              <div className="flex flex-col gap-1">
                {person.externalIds.map(e => (
                  <span key={e.source} className="text-xs text-white/40">
                    <span className="text-white/20">{e.source}:</span> {e.value}
                  </span>
                ))}
              </div>
            </div>
          )}
        </div>
      </aside>

      {/* Right panel */}
      <div className="flex-1 min-w-0 overflow-y-auto">
        <div className="px-8 pt-6">
          <Link to="/people" className="inline-flex items-center gap-1.5 text-sm text-white/40 hover:text-white/70 transition-colors">
            <ArrowLeft size={14} /> People
          </Link>
        </div>

        <div className="px-8 py-6 space-y-8">
          <div>
            <h1 className="text-3xl font-bold text-white mb-1">{person.name}</h1>
            <div className="flex flex-wrap items-center gap-2 mt-2">
              {person.monitored && <Badge color={ACCENT}>Monitored</Badge>}
              {itemsPage?.total !== undefined && (
                <span className="text-sm text-white/35">{itemsPage.total} items</span>
              )}
            </div>
          </div>

          {person.overview && (
            <section>
              <h2 className="text-xs font-semibold text-white/35 uppercase tracking-widest mb-3">Biography</h2>
              <p className="text-sm text-white/60 leading-relaxed max-w-3xl">{person.overview}</p>
            </section>
          )}

          {bands.length > 0 && (
            <section>
              <h2 className="text-xs font-semibold text-white/35 uppercase tracking-widest mb-4">Member of</h2>
              <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-4">
                {bands.map(band => (
                  <EntryCard
                    key={band.id}
                    entry={band}
                    href={`/music/${band.id}`}
                    accent={ACCENT}
                  />
                ))}
              </div>
            </section>
          )}

          {items.length > 0 && (
            <section>
              <h2 className="text-xs font-semibold text-white/35 uppercase tracking-widest mb-4">Filmography</h2>
              <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-4">
                {items.map(item => (
                  <ItemCard
                    key={item.id}
                    item={item}
                    href={item.contentType === 'adult' || item.contentType === 'jav'
                      ? `/afterdark/scenes/${item.id}`
                      : `/items/${item.id}`}
                    aspect="16/9"
                    accent={ACCENT}
                  />
                ))}
              </div>
            </section>
          )}
        </div>
      </div>
    </div>
  )
}
