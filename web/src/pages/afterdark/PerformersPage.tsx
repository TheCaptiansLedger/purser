import { useState } from 'react'
import { Users } from 'lucide-react'
import { usePeople } from '../../api/people'
import { PageHeader } from '../../components/layout/PageHeader'
import { PersonCard } from '../../components/media/PersonCard'
import { Pagination } from '../../components/ui/Pagination'
import { SkeletonGrid } from '../../components/ui/Skeleton'
import { EmptyState } from '../../components/ui/EmptyState'

const ACCENT = '#f43f5e'
const LIMIT = 48

export function PerformersPage() {
  const [search, setSearch] = useState('')
  const [offset, setOffset] = useState(0)

  const resetPage = (v: string) => { setSearch(v); setOffset(0) }

  const performers = usePeople({ contentType: 'adult,jav', search: search || undefined, limit: LIMIT, offset })

  const loading = performers.isLoading
  const all = performers.data?.data ?? []
  const total = performers.data?.total ?? 0

  return (
    <div>
      <PageHeader
        title="Performers"
        accent={ACCENT}
        search={search}
        onSearch={resetPage}
        total={total}
      />
      <div className="px-8 py-6">
        {loading ? (
          <SkeletonGrid count={24} aspect="2/3" />
        ) : !all.length ? (
          <EmptyState icon={Users} title="No performers yet" accent={ACCENT} />
        ) : (
          <>
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 2xl:grid-cols-8 gap-4">
              {all.map(p => (
                <PersonCard key={p.id} person={p} href={`/afterdark/performers/${p.id}`} accent={ACCENT} />
              ))}
            </div>
            <Pagination total={total} limit={LIMIT} offset={offset} onChange={setOffset} accent={ACCENT} />
          </>
        )}
      </div>
    </div>
  )
}
