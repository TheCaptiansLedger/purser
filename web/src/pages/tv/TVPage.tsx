import { useState } from 'react'
import { Tv2 } from 'lucide-react'
import { useLibraryEntries } from '../../api/library'
import { PageHeader } from '../../components/layout/PageHeader'
import { EntryCard } from '../../components/media/EntryCard'
import { Pagination } from '../../components/ui/Pagination'
import { SkeletonGrid } from '../../components/ui/Skeleton'
import { EmptyState } from '../../components/ui/EmptyState'

const ACCENT = '#8b5cf6'
const LIMIT = 48

export function TVPage() {
  const [search, setSearch] = useState('')
  const [offset, setOffset] = useState(0)

  const { data, isLoading } = useLibraryEntries({
    contentType: 'tv',
    kind: 'series',
    search: search || undefined,
    limit: LIMIT,
    offset,
  })

  return (
    <div>
      <PageHeader title="TV Shows" accent={ACCENT} search={search} onSearch={v => { setSearch(v); setOffset(0) }} total={data?.total} />
      <div className="px-8 py-6">
        {isLoading ? (
          <SkeletonGrid count={24} aspect="2/3" />
        ) : !data?.data.length ? (
          <EmptyState icon={Tv2} title="No TV shows yet" description="Add TV series to your library to see them here." accent={ACCENT} />
        ) : (
          <>
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 2xl:grid-cols-8 gap-4">
              {data.data.map(entry => (
                <EntryCard key={entry.id} entry={entry} href={`/tv/${entry.id}`} aspect="2/3" accent={ACCENT} />
              ))}
            </div>
            <Pagination total={data.total} limit={LIMIT} offset={offset} onChange={setOffset} accent={ACCENT} />
          </>
        )}
      </div>
    </div>
  )
}
