import { useState } from 'react'
import { BookMarked } from 'lucide-react'
import { useAllGroups } from '../../api/groups'
import { PageHeader } from '../../components/layout/PageHeader'
import { Pagination } from '../../components/ui/Pagination'
import { SkeletonGrid } from '../../components/ui/Skeleton'
import { EmptyState } from '../../components/ui/EmptyState'

const ACCENT = '#f59e0b'
const LIMIT = 48

export function BookSeriesPage() {
  const [search, setSearch] = useState('')
  const [offset, setOffset] = useState(0)

  const { data, isLoading } = useAllGroups({
    contentType: 'book',
    search: search || undefined,
    limit: LIMIT,
    offset,
  })

  return (
    <div>
      <PageHeader
        title="Series"
        accent={ACCENT}
        search={search}
        onSearch={v => { setSearch(v); setOffset(0) }}
        total={data?.total}
      />
      <div className="px-8 py-6">
        {isLoading ? (
          <SkeletonGrid count={24} aspect="2/3" />
        ) : !data?.data.length ? (
          <EmptyState icon={BookMarked} title="No series yet" description="Book series will appear here once added." accent={ACCENT} />
        ) : (
          <>
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4">
              {data.data.map(group => (
                <div
                  key={group.id}
                  className="group relative rounded-lg overflow-hidden bg-white/5 border border-white/8 hover:border-white/15 transition-all duration-200 cursor-pointer"
                >
                  <div className="aspect-[2/3] bg-white/5 flex items-center justify-center">
                    <BookMarked size={40} className="text-white/20" />
                  </div>
                  <div className="p-3">
                    <p className="text-sm font-medium text-white/90 truncate">{group.title}</p>
                    {group.number > 0 && (
                      <p className="text-xs text-white/40 mt-0.5">{group.number} books</p>
                    )}
                  </div>
                </div>
              ))}
            </div>
            <Pagination total={data.total} limit={LIMIT} offset={offset} onChange={setOffset} accent={ACCENT} />
          </>
        )}
      </div>
    </div>
  )
}
