import { useParams, Link } from 'react-router-dom'
import { Tag as TagIcon, Film, Tv, Music2, BookOpen, Video, Layers, FileVideo } from 'lucide-react'
import { useLibraryEntries } from '../../api/library'
import { useAllGroups } from '../../api/groups'
import { useItems } from '../../api/items'
import { useLibraryEntry } from '../../api/library'
import { EmptyState } from '../../components/ui/EmptyState'
import { Skeleton } from '../../components/ui/Skeleton'
import type { LibraryEntry, Group, Item, ContentType } from '../../types'

const ACCENT = '#6366f1'

function entryLink(entry: LibraryEntry): string {
  switch (entry.contentType) {
    case 'movie':  return `/movies/${entry.id}`
    case 'tv':     return `/tv/${entry.id}`
    case 'music':  return `/music/${entry.id}`
    case 'book':   return `/books/${entry.id}`
    case 'adult':
    case 'jav':    return `/afterdark/studios/${entry.id}`
    default:       return '#'
  }
}

function itemLink(item: Item): string {
  switch (item.contentType) {
    case 'adult':
    case 'jav':   return `/afterdark/scenes/${item.id}`
    case 'movie': return item.libraryEntryId ? `/movies/${item.libraryEntryId}` : '#'
    case 'tv':    return item.libraryEntryId ? `/tv/${item.libraryEntryId}` : '#'
    case 'music': return item.libraryEntryId ? `/music/${item.libraryEntryId}` : '#'
    case 'book':  return item.libraryEntryId ? `/books/${item.libraryEntryId}` : '#'
    default:      return '#'
  }
}

function contentTypeIcon(ct: ContentType) {
  switch (ct) {
    case 'movie':  return Film
    case 'tv':     return Tv
    case 'music':  return Music2
    case 'book':   return BookOpen
    case 'adult':
    case 'jav':    return Video
    default:       return Layers
  }
}

function contentTypeLabel(ct: ContentType): string {
  switch (ct) {
    case 'movie': return 'Movie'
    case 'tv':    return 'TV Series'
    case 'music': return 'Artist'
    case 'book':  return 'Book'
    case 'adult': return 'Studio'
    case 'jav':   return 'Studio'
    default:      return ct
  }
}

function itemTypeLabel(ct: ContentType): string {
  switch (ct) {
    case 'movie': return 'Movie'
    case 'tv':    return 'Episode'
    case 'music': return 'Track'
    case 'book':  return 'Book'
    case 'adult':
    case 'jav':   return 'Scene'
    default:      return 'Item'
  }
}

function groupLink(group: Group, parentEntry: LibraryEntry | undefined): string {
  if (!parentEntry) return '#'
  switch (parentEntry.contentType) {
    case 'music': return `/music/${group.libraryEntryId}/albums/${group.id}`
    case 'tv':    return `/tv/${group.libraryEntryId}/seasons/${group.number}`
    default:      return '#'
  }
}

function EntryRow({ entry }: { entry: LibraryEntry }) {
  const Icon = contentTypeIcon(entry.contentType)
  return (
    <Link
      to={entryLink(entry)}
      className="flex items-center gap-3 px-4 py-3 rounded-xl bg-white/3 border border-white/5 hover:bg-white/6 hover:border-white/12 transition-all duration-150 group"
    >
      {entry.imageUrl ? (
        <img src={entry.imageUrl} alt={entry.name} className="w-10 h-10 rounded-lg object-cover shrink-0" />
      ) : (
        <div className="w-10 h-10 rounded-lg bg-white/5 flex items-center justify-center shrink-0">
          <Icon size={18} className="text-white/25" />
        </div>
      )}
      <div className="flex-1 min-w-0">
        <p className="text-sm font-medium text-white/80 group-hover:text-white transition-colors truncate">{entry.name}</p>
        <p className="text-xs text-white/35">{contentTypeLabel(entry.contentType)}</p>
      </div>
    </Link>
  )
}

function GroupRow({ group }: { group: Group }) {
  // N+1: one useLibraryEntry call per group in the result set. Acceptable here
  // because tag browse returns at most 100 groups and React Query deduplicates
  // identical requests. Track as Option A in #223 if this becomes a problem.
  const { data: parent } = useLibraryEntry(group.libraryEntryId)
  const href = groupLink(group, parent)

  const inner = (
    <div className="flex items-center gap-3 px-4 py-3 rounded-xl bg-white/3 border border-white/5 hover:bg-white/6 hover:border-white/12 transition-all duration-150 group">
      {group.coverUrl ? (
        <img src={group.coverUrl} alt={group.title} className="w-10 h-10 rounded-lg object-cover shrink-0" />
      ) : (
        <div className="w-10 h-10 rounded-lg bg-white/5 flex items-center justify-center shrink-0">
          <Layers size={18} className="text-white/25" />
        </div>
      )}
      <div className="flex-1 min-w-0">
        <p className="text-sm font-medium text-white/80 group-hover:text-white transition-colors truncate">{group.title}</p>
        <p className="text-xs text-white/35">
          {parent ? `${contentTypeLabel(parent.contentType as ContentType)} — ${parent.name}` : 'Album'}
          {group.year > 0 ? ` · ${group.year}` : ''}
        </p>
      </div>
    </div>
  )

  return href === '#' ? <div>{inner}</div> : <Link to={href}>{inner}</Link>
}

function ItemRow({ item }: { item: Item }) {
  const href = itemLink(item)
  const Icon = contentTypeIcon(item.contentType)

  const inner = (
    <div className="flex items-center gap-3 px-4 py-3 rounded-xl bg-white/3 border border-white/5 hover:bg-white/6 hover:border-white/12 transition-all duration-150 group">
      {item.coverUrl ? (
        <img src={item.coverUrl} alt={item.title} className="w-10 h-10 rounded-lg object-cover shrink-0" />
      ) : (
        <div className="w-10 h-10 rounded-lg bg-white/5 flex items-center justify-center shrink-0">
          <FileVideo size={18} className="text-white/25" />
        </div>
      )}
      <div className="flex-1 min-w-0">
        <p className="text-sm font-medium text-white/80 group-hover:text-white transition-colors truncate">{item.title}</p>
        <p className="text-xs text-white/35">{itemTypeLabel(item.contentType)}</p>
      </div>
      <Icon size={14} className="text-white/20 shrink-0" />
    </div>
  )

  return href === '#' ? <div>{inner}</div> : <Link to={href}>{inner}</Link>
}

function ResultSection<T extends { id: string }>({
  label,
  items,
  renderRow,
}: {
  label: string
  items: T[]
  renderRow: (item: T) => React.ReactNode
}) {
  if (items.length === 0) return null
  return (
    <section>
      <h2 className="text-sm font-semibold text-white/40 uppercase tracking-widest mb-3">
        {label}
        <span className="ml-2 font-normal normal-case tracking-normal text-white/20">{items.length}</span>
      </h2>
      <div className="space-y-1.5">
        {items.map((item) => <div key={item.id}>{renderRow(item)}</div>)}
      </div>
    </section>
  )
}

export function TagBrowsePage() {
  const { key, value } = useParams<{ key: string; value: string }>()
  const decodedKey   = decodeURIComponent(key ?? '')
  const decodedValue = decodeURIComponent(value ?? '')

  const entriesQ = useLibraryEntries({ tag_key: decodedKey, tag_value: decodedValue, limit: 100 })
  const groupsQ  = useAllGroups({ tag_key: decodedKey, tag_value: decodedValue, limit: 100 })
  const itemsQ   = useItems({ tag_key: decodedKey, tag_value: decodedValue, limit: 100 })

  const entries = entriesQ.data?.data ?? []
  const groups  = groupsQ.data?.data ?? []
  const items   = itemsQ.data?.data ?? []
  const isLoading = entriesQ.isLoading || groupsQ.isLoading || itemsQ.isLoading
  const isEmpty = !isLoading && entries.length === 0 && groups.length === 0 && items.length === 0

  return (
    <div className="px-8 py-8">
      <div className="mb-8">
        <div className="flex items-center gap-2 mb-1">
          <TagIcon size={16} style={{ color: ACCENT }} />
          <span className="text-xs font-semibold uppercase tracking-widest" style={{ color: ACCENT }}>{decodedKey}</span>
        </div>
        <h1 className="text-3xl font-bold text-white">{decodedValue}</h1>
      </div>

      {isLoading ? (
        <div className="space-y-2">
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-16 w-full" />
          ))}
        </div>
      ) : isEmpty ? (
        <EmptyState icon={TagIcon} title={`No content tagged ${decodedKey}: ${decodedValue}`} accent={ACCENT} />
      ) : (
        <div className="space-y-10">
          <ResultSection
            label="Entries"
            items={entries}
            renderRow={e => <EntryRow entry={e} />}
          />
          <ResultSection
            label="Albums & Groups"
            items={groups}
            renderRow={g => <GroupRow group={g} />}
          />
          <ResultSection
            label="Items"
            items={items}
            renderRow={i => <ItemRow item={i} />}
          />
        </div>
      )}
    </div>
  )
}
