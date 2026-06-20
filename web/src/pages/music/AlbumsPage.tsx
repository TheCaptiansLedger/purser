import { useState } from 'react'
import { Disc3 } from 'lucide-react'
import { Link } from 'react-router-dom'
import { useAllGroups } from '../../api/groups'
import { PageHeader } from '../../components/layout/PageHeader'
import { Pagination } from '../../components/ui/Pagination'
import { SkeletonGrid } from '../../components/ui/Skeleton'
import { EmptyState } from '../../components/ui/EmptyState'

const ACCENT = '#10b981'
const LIMIT = 48

export function AlbumsPage() {
  const [search, setSearch] = useState('')
  const [offset, setOffset] = useState(0)

  const { data, isLoading } = useAllGroups({
    contentType: 'music',
    search: search || undefined,
    limit: LIMIT,
    offset,
  })

  return (
    <div>
      <PageHeader
        title="Albums"
        accent={ACCENT}
        search={search}
        onSearch={v => { setSearch(v); setOffset(0) }}
        total={data?.total}
      />
      <div className="px-8 py-6">
        {isLoading ? (
          <SkeletonGrid count={24} aspect="1/1" />
        ) : !data?.data.length ? (
          <EmptyState icon={Disc3} title="No albums yet" description="Add music to your library to see albums here." accent={ACCENT} />
        ) : (
          <>
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4">
              {data.data.map(group => (
                <Link
                  key={group.id}
                  to={`/music/${group.libraryEntryId}/albums/${group.id}`}
                  className="group relative rounded-lg overflow-hidden bg-white/5 border border-white/8 hover:border-white/15 transition-all duration-200"
                >
                  <div className="aspect-square bg-white/5 flex items-center justify-center overflow-hidden">
                    {group.coverUrl ? (
                      <img src={group.coverUrl} alt={group.title} className="w-full h-full object-cover" />
                    ) : (
                      <Disc3 size={48} className="text-white/20" />
                    )}
                  </div>
                  <div className="p-3">
                    <p className="text-sm font-medium text-white/90 truncate">{group.title}</p>
                    {group.year > 0 && (
                      <p className="text-xs text-white/40 mt-0.5">{group.year}</p>
                    )}
                  </div>
                </Link>
              ))}
            </div>
            <Pagination total={data.total} limit={LIMIT} offset={offset} onChange={setOffset} accent={ACCENT} />
          </>
        )}
      </div>
    </div>
  )
}
