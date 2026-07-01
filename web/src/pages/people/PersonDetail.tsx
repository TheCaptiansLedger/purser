import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { ArrowLeft, User } from 'lucide-react'
import { usePerson } from '../../api/people'
import { useItems } from '../../api/items'
import { useLibraryEntries } from '../../api/library'
import { useImageVersion } from '../../hooks/useImageVersion'
import { EditButton } from '../../components/EditButton'
import { PersonEditor } from '../../components/edit/editors/PersonEditor'
import { Badge } from '../../components/ui/Badge'
import { EntryCard } from '../../components/media/EntryCard'
import { ItemCard } from '../../components/media/ItemCard'
import { PersonMetaGroups } from '../../components/media/PersonMetaGroups'
import { Lightbox } from '../../components/ui/Lightbox'
import { Skeleton } from '../../components/ui/Skeleton'
import { contentTypeConfig } from '../../config/contentTypes'
import type { ContentType, ItemStatus } from '../../types'
import { StatusFilterChips } from '../../components/media/StatusFilterChips'
import { ExpandableText } from '../../components/ui/ExpandableText'

const ACCENT = '#6366f1'

function groupBy<T>(items: T[], key: (item: T) => string): Record<string, T[]> {
  return items.reduce<Record<string, T[]>>((acc, item) => {
    const k = key(item)
    if (!acc[k]) acc[k] = []
    acc[k].push(item)
    return acc
  }, {})
}

export function PersonDetail() {
  const { id } = useParams<{ id: string }>()
  const [editOpen, setEditOpen] = useState(false)
  const [lightboxOpen, setLightboxOpen] = useState(false)
  const [statusFilter, setStatusFilter] = useState<ItemStatus | undefined>(undefined)
  const { data: person, isLoading } = usePerson(id!)
  const [versionedImageUrl, bumpImageVersion] = useImageVersion(person?.imageUrl)
  const { data: itemsPage }   = useItems({ personId: id!, status: statusFilter, limit: 48 })
  const { data: entriesPage } = useLibraryEntries({ personId: id! })

  const items         = itemsPage?.data   ?? []
  const linkedEntries = entriesPage?.data ?? []

  if (isLoading) return <div className="px-8 py-10"><Skeleton className="h-64 w-full" /></div>
  if (!person) return null

  const itemsByType   = groupBy(items, i => i.contentType)
  const entriesByType = groupBy(linkedEntries, e => e.contentType)

  return (
    <div className="flex min-h-screen">
      {/* Left panel */}
      <aside className="w-72 shrink-0 sticky top-0 h-screen overflow-y-auto flex flex-col border-r border-white/5">
        <div className="relative" style={{ aspectRatio: '2/3' }}>
          {versionedImageUrl ? (
            <button
              className="block w-full h-full cursor-zoom-in"
              onClick={() => setLightboxOpen(true)}
              aria-label={`View ${person.name} photo`}
            >
              <img src={versionedImageUrl} alt={person.name} className="w-full h-full object-cover object-top" />
            </button>
          ) : (
            <div className="w-full h-full bg-white/3 flex items-center justify-center">
              <User size={64} className="text-white/10" strokeWidth={1} />
            </div>
          )}
          <div className="absolute inset-0 bg-gradient-to-t from-[#08080e] via-transparent to-transparent pointer-events-none" />
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

          <PersonMetaGroups metadata={person.metadata} />

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
        <div className="px-8 pt-6 flex items-center justify-between">
          <Link to="/people" className="inline-flex items-center gap-1.5 text-sm text-white/40 hover:text-white/70 transition-colors">
            <ArrowLeft size={14} /> People
          </Link>
          <EditButton onClick={() => setEditOpen(true)} />
        </div>

        <div className="px-8 py-6 space-y-8">
          <div>
            <h1 className="text-3xl font-bold text-white mb-1">{person.name}</h1>
            <div className="flex flex-wrap items-center gap-2 mt-2">
              {person.monitored && <Badge color={ACCENT}>Monitored</Badge>}
              {itemsPage?.total !== undefined && itemsPage.total > 0 && (
                <span className="text-sm text-white/35">{itemsPage.total} appearance{itemsPage.total !== 1 ? 's' : ''}</span>
              )}
            </div>
          </div>

          {person.overview && (
            <section>
              <h2 className="text-xs font-semibold text-white/35 uppercase tracking-widest mb-3">Biography</h2>
              <ExpandableText text={person.overview} />
            </section>
          )}

          {/* Linked library entries — grouped by content type */}
          {Object.entries(entriesByType).map(([ct, entries]) => (
            <section key={ct}>
              <h2 className="text-xs font-semibold text-white/35 uppercase tracking-widest mb-4">
                {contentTypeConfig[ct as ContentType]?.entrySectionLabel ?? ct}
              </h2>
              <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-4">
                {entries.map(entry => (
                  <EntryCard
                    key={entry.id}
                    entry={entry}
                    href={contentTypeConfig[entry.contentType]?.entryPath(entry) ?? '/people'}
                    accent={ACCENT}
                  />
                ))}
              </div>
            </section>
          ))}

          {/* Item appearances — grouped by content type */}
          <div className="mb-4">
            <StatusFilterChips value={statusFilter} onChange={setStatusFilter} accent={ACCENT} />
          </div>
          {Object.entries(itemsByType).map(([ct, ctItems]) => (
            <section key={ct}>
              <h2 className="text-xs font-semibold text-white/35 uppercase tracking-widest mb-4">
                {contentTypeConfig[ct as ContentType]?.itemSectionLabel ?? ct}
              </h2>
              <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-4">
                {ctItems.map(item => (
                  <ItemCard
                    key={item.id}
                    item={item}
                    href={contentTypeConfig[item.contentType]?.itemPath(item) ?? '/people'}
                    aspect="16/9"
                    accent={ACCENT}
                    alwaysShowStatus={statusFilter !== undefined}
                  />
                ))}
              </div>
            </section>
          ))}
        </div>
      </div>

      {editOpen && (
        <PersonEditor
          person={person}
          onClose={() => setEditOpen(false)}
          onImageSet={bumpImageVersion}
        />
      )}

      {lightboxOpen && versionedImageUrl && (
        <Lightbox src={versionedImageUrl} alt={person.name} onClose={() => setLightboxOpen(false)} />
      )}
    </div>
  )
}
