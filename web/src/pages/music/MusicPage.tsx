import { useState } from 'react'
import { Music2 } from 'lucide-react'
import { useLibraryEntries } from '../../api/library'
import { PageHeader } from '../../components/layout/PageHeader'
import { EntryCard } from '../../components/media/EntryCard'
import { Pagination } from '../../components/ui/Pagination'
import { SkeletonGrid } from '../../components/ui/Skeleton'
import { EmptyState } from '../../components/ui/EmptyState'

const ACCENT = '#10b981'
const LIMIT = 48

export function MusicPage() {
  const [search, setSearch] = useState('')
  const [offset, setOffset] = useState(0)

  const { data, isLoading } = useLibraryEntries({
    contentType: 'music',
    kind: 'artist',
    search: search || undefined,
    limit: LIMIT,
    offset,
  })

  return (
    <div>
      <PageHeader title="Music" accent={ACCENT} search={search} onSearch={v => { setSearch(v); setOffset(0) }} total={data?.total} />
      <div className="px-8 py-6">
        {isLoading ? (
          <SkeletonGrid count={24} aspect="1/1" />
        ) : !data?.data.length ? (
          <EmptyState icon={Music2} title="No artists yet" description="Add music to your library to see artists here." accent={ACCENT} />
        ) : (
          <>
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4">
              {data.data.map(entry => (
                <EntryCard key={entry.id} entry={entry} href={`/music/${entry.id}`} aspect="1/1" accent={ACCENT} />
              ))}
            </div>
            <Pagination total={data.total} limit={LIMIT} offset={offset} onChange={setOffset} accent={ACCENT} />
          </>
        )}
      </div>
    </div>
  )
}
