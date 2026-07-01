import { useState } from 'react'
import { Film } from 'lucide-react'
import { useItems } from '../../api/items'
import type { ItemStatus } from '../../types'
import type { SortField, SortDir } from '../../api/items'
import { useStatusOverlay } from '../../hooks/useStatusOverlay'
import { StatusFilterChips } from '../../components/media/StatusFilterChips'
import { PageHeader } from '../../components/layout/PageHeader'
import { ItemCard } from '../../components/media/ItemCard'
import { Pagination } from '../../components/ui/Pagination'
import { SkeletonGrid } from '../../components/ui/Skeleton'
import { EmptyState } from '../../components/ui/EmptyState'

const ACCENT = '#f43f5e'
const LIMIT = 48

export function ScenesPage() {
  const [search, setSearch] = useState('')
  const [offset, setOffset] = useState(0)
  const [sort, setSort] = useState<SortField>('date')
  const [sortDir, setSortDir] = useState<SortDir>('desc')
  const [statusFilter, setStatusFilter] = useState<ItemStatus | undefined>(undefined)
  const [alwaysShowStatus, toggleStatus] = useStatusOverlay('afterdark')

  const resetPage = (v: string) => { setSearch(v); setOffset(0) }
  const changeStatusFilter = (s: ItemStatus | undefined) => { setStatusFilter(s); setOffset(0) }

  const changeSort = (newSort: SortField) => {
    if (newSort === sort) {
      setSortDir(d => d === 'desc' ? 'asc' : 'desc')
    } else {
      setSort(newSort)
      setSortDir(newSort === 'date' ? 'desc' : 'asc')
    }
    setOffset(0)
  }

  const scenes = useItems({
    contentType: 'adult,jav',
    search: search || undefined,
    sort,
    sortDir,
    status: statusFilter,
    limit: LIMIT,
    offset,
  })

  const loading = scenes.isLoading
  const allScenes = scenes.data?.data ?? []
  const total = scenes.data?.total ?? 0

  return (
    <div>
      <PageHeader
        title="Scenes"
        accent={ACCENT}
        search={search}
        onSearch={resetPage}
        total={total}
        statusOverlay={{ value: alwaysShowStatus, onToggle: toggleStatus }}
      />
      <div className="px-8 py-6">
        <div className="flex items-center gap-2 mb-2">
          <span className="text-xs text-white/30 uppercase tracking-widest mr-1">Sort</span>
          {([['date', 'Date'], ['title', 'A–Z']] as [SortField, string][]).map(([key, label]) => (
            <button
              key={key}
              onClick={() => changeSort(key)}
              className={[
                'text-xs px-3 py-1 rounded-lg border transition-colors',
                sort === key
                  ? 'border-transparent text-white'
                  : 'border-white/10 text-white/40 hover:text-white/70 hover:border-white/20',
              ].join(' ')}
              style={sort === key ? { background: ACCENT + '33', color: ACCENT, borderColor: ACCENT + '55' } : {}}
            >
              {label}{sort === key ? (sortDir === 'asc' ? ' ↑' : ' ↓') : ''}
            </button>
          ))}
        </div>
        <div className="mb-4">
          <StatusFilterChips value={statusFilter} onChange={changeStatusFilter} accent={ACCENT} />
        </div>
        {loading ? (
          <SkeletonGrid count={24} aspect="16/9" />
        ) : !allScenes.length ? (
          <EmptyState icon={Film} title="No scenes yet" accent={ACCENT} />
        ) : (
          <>
            <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 2xl:grid-cols-6 gap-4">
              {allScenes.map(item => (
                <ItemCard
                  key={item.id}
                  item={item}
                  href={`/afterdark/scenes/${item.id}`}
                  aspect="16/9"
                  accent={ACCENT}
                  showPeople
                  alwaysShowStatus={alwaysShowStatus || statusFilter !== undefined}
                />
              ))}
            </div>
            <Pagination total={total} limit={LIMIT} offset={offset} onChange={setOffset} accent={ACCENT} />
          </>
        )}
      </div>
    </div>
  )
}
