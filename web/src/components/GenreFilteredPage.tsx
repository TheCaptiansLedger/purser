import { useState } from 'react'
import { useParams } from 'react-router-dom'
import type { LucideIcon } from 'lucide-react'
import type { ContentType, Kind, LibraryEntry } from '../types'
import { useLibraryEntries } from '../api/library'
import { slugToLabel } from '../utils/toSlug'
import { PageHeader } from './layout/PageHeader'
import { EntryCard } from './media/EntryCard'
import { Pagination } from './ui/Pagination'
import { SkeletonGrid } from './ui/Skeleton'
import { EmptyState } from './ui/EmptyState'

const LIMIT = 48

interface GenreFilteredPageProps {
  contentType: ContentType
  kind: Kind
  accent: string
  toEntryHref: (entry: LibraryEntry) => string
  icon: LucideIcon
  emptyNoun: string
}

export function GenreFilteredPage({ contentType, kind, accent, toEntryHref, icon: Icon, emptyNoun }: GenreFilteredPageProps) {
  const { genre = '' } = useParams<{ genre: string }>()
  const [search, setSearch] = useState('')
  const [offset, setOffset] = useState(0)

  const label = slugToLabel(genre)

  const { data, isLoading } = useLibraryEntries({
    contentType,
    kind,
    tag_key: 'genre',
    tag_value: label,
    search: search || undefined,
    limit: LIMIT,
    offset,
  })

  return (
    <div>
      <PageHeader
        title={label}
        accent={accent}
        search={search}
        onSearch={v => { setSearch(v); setOffset(0) }}
        total={data?.total}
      />
      <div className="px-8 py-6">
        {isLoading ? (
          <SkeletonGrid count={24} aspect="2/3" />
        ) : !data?.data.length ? (
          <EmptyState icon={Icon} title={`No ${label} ${emptyNoun}`} accent={accent} />
        ) : (
          <>
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4">
              {data.data.map(entry => (
                <EntryCard key={entry.id} entry={entry} href={toEntryHref(entry)} aspect="2/3" accent={accent} />
              ))}
            </div>
            <Pagination total={data.total} limit={LIMIT} offset={offset} onChange={setOffset} accent={accent} />
          </>
        )}
      </div>
    </div>
  )
}
