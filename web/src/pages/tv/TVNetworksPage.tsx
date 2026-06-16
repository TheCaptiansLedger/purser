import { useState } from 'react'
import { Globe } from 'lucide-react'
import { useLibraryEntries } from '../../api/library'
import { PageHeader } from '../../components/layout/PageHeader'
import { EntryCard } from '../../components/media/EntryCard'
import { Pagination } from '../../components/ui/Pagination'
import { SkeletonGrid } from '../../components/ui/Skeleton'
import { EmptyState } from '../../components/ui/EmptyState'

const ACCENT = '#8b5cf6'
const LIMIT = 48

export function TVNetworksPage() {
  const [search, setSearch] = useState('')
  const [offset, setOffset] = useState(0)

  const { data, isLoading } = useLibraryEntries({
    contentType: 'tv',
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
        onSearch={v => { setSearch(v); setOffset(0) }}
        total={data?.total}
      />
      <div className="px-8 py-6">
        {isLoading ? (
          <SkeletonGrid count={24} aspect="16/9" />
        ) : !data?.data.length ? (
          <EmptyState icon={Globe} title="No networks yet" description="Add TV networks to your library to see them here." accent={ACCENT} />
        ) : (
          <>
            <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-4">
              {data.data.map(entry => (
                <EntryCard key={entry.id} entry={entry} href={`/tv/${entry.id}`} aspect="16/9" accent={ACCENT} />
              ))}
            </div>
            <Pagination total={data.total} limit={LIMIT} offset={offset} onChange={setOffset} accent={ACCENT} />
          </>
        )}
      </div>
    </div>
  )
}
