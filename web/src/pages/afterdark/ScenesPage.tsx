import { useState } from 'react'
import { Film } from 'lucide-react'
import { useItems } from '../../api/items'
import { useStatusOverlay } from '../../hooks/useStatusOverlay'
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
  const [alwaysShowStatus, toggleStatus] = useStatusOverlay('afterdark')

  const resetPage = (v: string) => { setSearch(v); setOffset(0) }

  const adultScenes = useItems({
    contentType: 'adult',
    search: search || undefined,
    limit: LIMIT,
    offset,
  })

  const javScenes = useItems({
    contentType: 'jav',
    search: search || undefined,
    limit: LIMIT,
    offset,
  })

  const loading = adultScenes.isLoading || javScenes.isLoading

  // Merge adult + jav, interleaved by date (newest first)
  const allScenes = [
    ...(adultScenes.data?.data ?? []),
    ...(javScenes.data?.data ?? []),
  ].sort((a, b) => {
    const da = a.date ? new Date(a.date).getTime() : 0
    const db = b.date ? new Date(b.date).getTime() : 0
    return db - da
  })

  const total = (adultScenes.data?.total ?? 0) + (javScenes.data?.total ?? 0)

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
        {loading ? (
          <SkeletonGrid count={24} aspect="16/9" />
        ) : !allScenes.length ? (
          <EmptyState icon={Film} title="No scenes yet" accent={ACCENT} />
        ) : (
          <>
            <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-4">
              {allScenes.map(item => (
                <ItemCard
                  key={item.id}
                  item={item}
                  href={`/afterdark/scenes/${item.id}`}
                  aspect="16/9"
                  accent={ACCENT}
                  showPeople
                  alwaysShowStatus={alwaysShowStatus}
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
