import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { ArrowLeft, Disc3 } from 'lucide-react'
import { useAllGroups, sortGroupsByYear } from '../../api/groups'
import { PageHeader } from '../../components/layout/PageHeader'
import { Pagination } from '../../components/ui/Pagination'
import { SkeletonGrid } from '../../components/ui/Skeleton'
import { EmptyState } from '../../components/ui/EmptyState'

const ACCENT = '#10b981'
const LIMIT = 48

export function LabelAlbumsPage() {
  const { label = '' } = useParams<{ label: string }>()
  const [search, setSearch] = useState('')
  const [offset, setOffset] = useState(0)

  const decoded = decodeURIComponent(label)

  const { data, isLoading } = useAllGroups({
    contentType: 'music',
    tag_key: 'label',
    tag_value: decoded,
    search: search || undefined,
    limit: LIMIT,
    offset,
  })

  const sorted = data ? sortGroupsByYear(data.data, 'desc') : []

  return (
    <div>
      <div className="px-8 pt-6 mb-2">
        <Link to="/music/labels" className="inline-flex items-center gap-1.5 text-sm text-white/40 hover:text-white/70 transition-colors">
          <ArrowLeft size={14} /> Labels
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
          <SkeletonGrid count={24} aspect="1/1" />
        ) : !sorted.length ? (
          <EmptyState icon={Disc3} title={`No albums for ${decoded}`} accent={ACCENT} />
        ) : (
          <>
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4">
              {sorted.map(group => (
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
            <Pagination total={data!.total} limit={LIMIT} offset={offset} onChange={setOffset} accent={ACCENT} />
          </>
        )}
      </div>
    </div>
  )
}
