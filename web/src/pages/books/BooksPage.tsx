import { useState } from 'react'
import { BookOpen } from 'lucide-react'
import { useLibraryEntries } from '../../api/library'
import { PageHeader } from '../../components/layout/PageHeader'
import { EntryCard } from '../../components/media/EntryCard'
import { Pagination } from '../../components/ui/Pagination'
import { SkeletonGrid } from '../../components/ui/Skeleton'
import { EmptyState } from '../../components/ui/EmptyState'

const ACCENT = '#f59e0b'
const LIMIT = 48

export function BooksPage() {
  const [search, setSearch] = useState('')
  const [offset, setOffset] = useState(0)

  const { data, isLoading } = useLibraryEntries({
    contentType: 'book',
    search: search || undefined,
    limit: LIMIT,
    offset,
  })

  return (
    <div>
      <PageHeader title="Books" accent={ACCENT} search={search} onSearch={v => { setSearch(v); setOffset(0) }} total={data?.total} />
      <div className="px-8 py-6">
        {isLoading ? (
          <SkeletonGrid count={24} aspect="2/3" />
        ) : !data?.data.length ? (
          <EmptyState icon={BookOpen} title="No books yet" description="Add books to your library to see them here." accent={ACCENT} />
        ) : (
          <>
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4">
              {data.data.map(entry => (
                <EntryCard key={entry.id} entry={entry} href={`/books/${entry.id}`} aspect="2/3" accent={ACCENT} />
              ))}
            </div>
            <Pagination total={data.total} limit={LIMIT} offset={offset} onChange={setOffset} accent={ACCENT} />
          </>
        )}
      </div>
    </div>
  )
}
