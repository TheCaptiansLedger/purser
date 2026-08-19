import { AlertTriangle, CheckCircle2, Download, ListMusic } from 'lucide-react'
import { useMemo, useState } from 'react'
import type { LucideIcon } from 'lucide-react'
import { Link } from 'react-router-dom'
import { EmptyState } from '../components/EmptyState'
import { ItemStatusBadge } from '../components/ItemStatusBadge'
import { ItemStatus } from '../gen/purser/domain/v1/common_pb'
import { useGroupsByIds } from '../hooks/useGroupsByIds'
import { useItemsByStatus } from '../hooks/useItemsByStatus'
import { useLibraryEntriesByIds } from '../hooks/useLibraryEntriesByIds'
import { itemStatusFromProto } from '../hooks/useReleaseTracks'

// WantedBoardTab — one status bucket per tab, each backed by its own
// ItemService.ListItems(content_type="music", status=X) call (#648's
// filter) rather than a client-side split of one unfiltered load — see
// this issue's own acceptance criterion.
type WantedBoardTab = 'wanted' | 'grabbed' | 'downloading' | 'missing'

const TAB_ITEMS: { id: WantedBoardTab; label: string; status: ItemStatus; emptyIcon: LucideIcon }[] = [
  { id: 'wanted', label: 'Wanted', status: ItemStatus.WANTED, emptyIcon: ListMusic },
  { id: 'grabbed', label: 'Grabbed', status: ItemStatus.GRABBED, emptyIcon: CheckCircle2 },
  { id: 'downloading', label: 'Downloading', status: ItemStatus.DOWNLOADING, emptyIcon: Download },
  { id: 'missing', label: 'Missing', status: ItemStatus.MISSING, emptyIcon: AlertTriangle },
]

function unique(ids: string[]): string[] {
  return Array.from(new Set(ids.filter(id => id !== '')))
}

// WantedBoard — the Wanted board (#678). Status-bucket tabs, each Item
// resolved to its Group/LibraryEntry via Get for display (artist name,
// album title), batched/deduped per page rather than one call per row —
// see docs/technical/music-web-ui.md's "Wanted board" section.
export function WantedBoard() {
  const [activeTab, setActiveTab] = useState<WantedBoardTab>('wanted')
  const activeStatus = TAB_ITEMS.find(tab => tab.id === activeTab)!.status

  const { data, isPending, isError, error, hasNextPage, isFetchingNextPage, fetchNextPage } =
    useItemsByStatus(activeStatus)

  const items = useMemo(() => data?.pages.flatMap(page => page.items) ?? [], [data])

  const groupIds = useMemo(() => unique(items.map(item => item.groupId)), [items])
  const libraryEntryIds = useMemo(() => unique(items.map(item => item.libraryEntryId)), [items])
  const { groupsById } = useGroupsByIds(groupIds)
  const { entriesById } = useLibraryEntriesByIds(libraryEntryIds)

  const isEmpty = !isPending && !isError && items.length === 0
  const activeTabItem = TAB_ITEMS.find(tab => tab.id === activeTab)!

  return (
    <div className="px-6 py-10 md:px-8">
      <h1 className="text-headline text-text">Wanted</h1>

      <div className="mt-6" role="tablist" aria-label="Wanted board">
        <div className="flex gap-1 border-b border-border">
          {TAB_ITEMS.map(tab => (
            <button
              key={tab.id}
              type="button"
              role="tab"
              aria-selected={activeTab === tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={[
                'px-3 h-10 flex items-center text-body border-b-2 -mb-px transition-colors',
                'text-text-secondary hover:text-text',
                activeTab === tab.id ? 'text-text border-text' : 'border-transparent',
              ].join(' ')}
            >
              {tab.label}
            </button>
          ))}
        </div>

        <div role="tabpanel" className="mt-6">
          {isError && (
            <p className="text-body text-status-failure" role="alert">
              Couldn't load {activeTabItem.label} ({error.message}).
            </p>
          )}

          {isEmpty && (
            <EmptyState
              icon={activeTabItem.emptyIcon}
              title={`Nothing ${activeTabItem.label}`}
              description={`No tracks are currently ${activeTabItem.label.toLowerCase()}.`}
            />
          )}

          {items.length > 0 && (
            <ul className="flex flex-col divide-y divide-border">
              {items.map(item => {
                const artist = entriesById[item.libraryEntryId]
                const album = groupsById[item.groupId]
                const status = itemStatusFromProto(item.status)
                return (
                  <li key={item.id} className="flex items-center gap-3 py-2">
                    {artist ? (
                      <Link to={`/music/artists/${artist.id}`} className="w-40 shrink-0 truncate text-body text-text hover:underline">
                        {artist.name}
                      </Link>
                    ) : (
                      <span className="w-40 shrink-0 truncate text-body text-text-secondary">—</span>
                    )}
                    {album ? (
                      <Link to={`/music/albums/${album.id}`} className="w-48 shrink-0 truncate text-body text-text hover:underline">
                        {album.title}
                      </Link>
                    ) : (
                      <span className="w-48 shrink-0 truncate text-body text-text-secondary">—</span>
                    )}
                    <span className="flex-1 truncate text-body text-text-secondary">{item.title}</span>
                    {status && <ItemStatusBadge status={status} />}
                  </li>
                )
              })}
            </ul>
          )}

          {hasNextPage && (
            <div className="mt-6 flex justify-center">
              <button
                type="button"
                onClick={() => fetchNextPage()}
                disabled={isFetchingNextPage}
                className="h-9 px-4 rounded-lg bg-surface-raised border border-border text-body font-medium text-text hover:bg-surface disabled:opacity-50"
              >
                {isFetchingNextPage ? 'Loading…' : 'Load more'}
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
