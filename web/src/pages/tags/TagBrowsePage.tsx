import { useParams, Link } from 'react-router-dom'
import { Tag as TagIcon, Layers, FileVideo } from 'lucide-react'
import { useLibraryEntries } from '../../api/library'
import { useAllGroups } from '../../api/groups'
import { useItems } from '../../api/items'
import { useLibraryEntry } from '../../api/library'
import { EmptyState } from '../../components/ui/EmptyState'
import { Skeleton } from '../../components/ui/Skeleton'
import { contentTypeConfig, defaultIcon } from '../../config/contentTypes'
import type { LibraryEntry, Group, Item, ContentType } from '../../types'

const ACCENT = '#6366f1'

function EntryRow({ entry }: { entry: LibraryEntry }) {
  const cfg  = contentTypeConfig[entry.contentType]
  const Icon = cfg?.icon ?? defaultIcon
  return (
    <Link
      to={cfg?.entryPath(entry) ?? '#'}
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
        <p className="text-xs text-white/35">{cfg?.entryTypeLabel ?? entry.contentType}</p>
      </div>
    </Link>
  )
}

function GroupRow({ group }: { group: Group }) {
  // N+1: one useLibraryEntry call per group in the result set. Acceptable here
  // because tag browse returns at most 100 groups and React Query deduplicates
  // identical requests. Track as Option A in #223 if this becomes a problem.
  const { data: parent } = useLibraryEntry(group.libraryEntryId)
  const href = parent
    ? contentTypeConfig[parent.contentType as ContentType]?.groupPath?.(group) ?? '#'
    : '#'

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
          {parent ? `${contentTypeConfig[parent.contentType as ContentType]?.entryTypeLabel ?? parent.contentType} — ${parent.name}` : 'Album'}
          {group.year > 0 ? ` · ${group.year}` : ''}
        </p>
      </div>
    </div>
  )

  return href === '#' ? <div>{inner}</div> : <Link to={href}>{inner}</Link>
}

function ItemRow({ item }: { item: Item }) {
  const cfg  = contentTypeConfig[item.contentType]
  const href = cfg?.itemPath(item) ?? '#'
  const Icon = cfg?.icon ?? defaultIcon

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
        <p className="text-xs text-white/35">{cfg?.itemTypeLabel ?? 'Item'}</p>
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
