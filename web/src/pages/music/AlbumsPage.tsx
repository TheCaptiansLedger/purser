import { useState } from 'react'
import { Disc3, ArrowUpNarrowWide, ArrowDownNarrowWide } from 'lucide-react'
import { useAllGroups, sortGroupsByYear } from '../../api/groups'
import type { YearSortDir } from '../../api/groups'
import { AlbumCard } from '../../components/AlbumCard'
import { PageHeader } from '../../components/layout/PageHeader'
import { Pagination } from '../../components/ui/Pagination'
import { SkeletonGrid } from '../../components/ui/Skeleton'
import { EmptyState } from '../../components/ui/EmptyState'

const ACCENT = '#10b981'
const LIMIT = 48

export function AlbumsPage() {
  const [search, setSearch] = useState('')
  const [offset, setOffset] = useState(0)
  const [sortDir, setSortDir] = useState<YearSortDir>('desc')

  const { data, isLoading } = useAllGroups({
    contentType: 'music',
    search: search || undefined,
    limit: LIMIT,
    offset,
  })

  const sorted = data ? sortGroupsByYear(data.data, sortDir) : []

  return (
    <div>
      <PageHeader
        title="Albums"
        accent={ACCENT}
        search={search}
        onSearch={v => { setSearch(v); setOffset(0) }}
        total={data?.total}
      >
        <button
          type="button"
          onClick={() => setSortDir(d => d === 'asc' ? 'desc' : 'asc')}
          title={sortDir === 'asc' ? 'Oldest first — click for newest first' : 'Newest first — click for oldest first'}
          className="inline-flex items-center gap-1.5 text-xs px-2.5 py-1.5 rounded-lg border transition-colors shrink-0"
          style={{ borderColor: 'rgba(255,255,255,0.1)', color: 'rgba(255,255,255,0.4)' }}
        >
          {sortDir === 'asc' ? <ArrowUpNarrowWide size={13} /> : <ArrowDownNarrowWide size={13} />}
          {sortDir === 'asc' ? 'Oldest first' : 'Newest first'}
        </button>
      </PageHeader>
      <div className="px-8 py-6">
        {isLoading ? (
          <SkeletonGrid count={24} aspect="1/1" />
        ) : !sorted.length ? (
          <EmptyState icon={Disc3} title="No albums yet" description="Add music to your library to see albums here." accent={ACCENT} />
        ) : (
          <>
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4">
              {sorted.map(group => (
                <AlbumCard
                  key={group.id}
                  album={group}
                  href={`/music/${group.libraryEntryId}/albums/${group.id}`}
                />
              ))}
            </div>
            <Pagination total={data!.total} limit={LIMIT} offset={offset} onChange={setOffset} accent={ACCENT} />
          </>
        )}
      </div>
    </div>
  )
}
