import { useState } from 'react'
import { Globe } from 'lucide-react'
import { useLibraryEntries } from '../../api/library'
import { PageHeader } from '../../components/layout/PageHeader'
import { EntryCard } from '../../components/media/EntryCard'
import { Pagination } from '../../components/ui/Pagination'
import { SkeletonGrid } from '../../components/ui/Skeleton'
import { EmptyState } from '../../components/ui/EmptyState'

const ACCENT = '#f43f5e'
const LIMIT = 48

export function NetworksPage() {
  const [search, setSearch] = useState('')
  const [offset, setOffset] = useState(0)

  const resetPage = (v: string) => { setSearch(v); setOffset(0) }

  const networks = useLibraryEntries({
    kind: 'network',
    search: search || undefined,
    limit: LIMIT,
    offset,
  })

  return (
    <div>
      <PageHeader
        title="Networks"
        accent={ACCENT}
        search={search}
        onSearch={resetPage}
        total={networks.data?.total}
      />
      <div className="px-8 py-6">
        {networks.isLoading ? (
          <SkeletonGrid count={12} aspect="16/9" />
        ) : !networks.data?.data.length ? (
          <EmptyState icon={Globe} title="No networks yet" accent={ACCENT} />
        ) : (
          <>
            <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-4">
              {networks.data.data.map(e => (
                <EntryCard key={e.id} entry={e} href={`/afterdark/networks/${e.id}`} aspect="16/9" accent={ACCENT} />
              ))}
            </div>
            <Pagination total={networks.data.total} limit={LIMIT} offset={offset} onChange={setOffset} accent={ACCENT} />
          </>
        )}
      </div>
    </div>
  )
}
