import { useState } from 'react'
import { Users } from 'lucide-react'
import { usePeople } from '../../api/people'
import { PageHeader } from '../../components/layout/PageHeader'
import { PersonCard } from '../../components/media/PersonCard'
import { Pagination } from '../../components/ui/Pagination'
import { SkeletonGrid } from '../../components/ui/Skeleton'
import { EmptyState } from '../../components/ui/EmptyState'

const ACCENT = '#6366f1'
const LIMIT = 48

export function PeoplePage() {
  const [search, setSearch] = useState('')
  const [offset, setOffset] = useState(0)

  const { data, isLoading } = usePeople({
    search: search || undefined,
    limit: LIMIT,
    offset,
  })

  return (
    <div>
      <PageHeader title="People" accent={ACCENT} search={search} onSearch={v => { setSearch(v); setOffset(0) }} total={data?.total} />
      <div className="px-8 py-6">
        {isLoading ? (
          <SkeletonGrid count={24} aspect="2/3" />
        ) : !data?.data.length ? (
          <EmptyState icon={Users} title="No people yet" description="People appear here when they are linked to items in your library." accent={ACCENT} />
        ) : (
          <>
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4">
              {data.data.map(p => (
                <PersonCard key={p.id} person={p} href={`/people/${p.id}`} accent={ACCENT} />
              ))}
            </div>
            <Pagination total={data.total} limit={LIMIT} offset={offset} onChange={setOffset} accent={ACCENT} />
          </>
        )}
      </div>
    </div>
  )
}
