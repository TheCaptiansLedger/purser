import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { ArrowLeft, Tv2 } from 'lucide-react'
import { useLibraryEntries } from '../../api/library'
import { PageHeader } from '../../components/layout/PageHeader'
import { EntryCard } from '../../components/media/EntryCard'
import { Pagination } from '../../components/ui/Pagination'
import { SkeletonGrid } from '../../components/ui/Skeleton'
import { EmptyState } from '../../components/ui/EmptyState'

const ACCENT = '#8b5cf6'
const LIMIT = 48

export function TVNetworkSeriesPage() {
  const { network = '' } = useParams<{ network: string }>()
  const [search, setSearch] = useState('')
  const [offset, setOffset] = useState(0)

  const decoded = decodeURIComponent(network)

  const { data, isLoading } = useLibraryEntries({
    contentType: 'tv',
    kind: 'series',
    tag_key: 'network',
    tag_value: decoded,
    search: search || undefined,
    limit: LIMIT,
    offset,
  })

  return (
    <div>
      <div className="px-8 pt-6 mb-2">
        <Link to="/tv/networks" className="inline-flex items-center gap-1.5 text-sm text-white/40 hover:text-white/70 transition-colors">
          <ArrowLeft size={14} /> Networks
        </Link>
      </div>
      <PageHeader
        title={decoded}
        accent={ACCENT}
        search={search}
        onSearch={v => { setSearch(v); setOffset(0) }}
        total={data?.total}
      />
      <div className="px-8 py-6">
        {isLoading ? (
          <SkeletonGrid count={24} aspect="2/3" />
        ) : !data?.data.length ? (
          <EmptyState icon={Tv2} title={`No shows on ${decoded}`} accent={ACCENT} />
        ) : (
          <>
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4">
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
