import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import type { LucideIcon } from 'lucide-react'
import { useTags } from '../api/tags'
import { filterTags } from './TagCloudPage'
import { toSlug } from '../utils/toSlug'
import { PageHeader } from './layout/PageHeader'
import { EmptyState } from './ui/EmptyState'

interface GenreListPageProps {
  contentType: string
  accent: string
  toEntryHref: (slug: string) => string
  icon: LucideIcon
  emptyDescription?: string
}

export function GenreListPage({ contentType, accent, toEntryHref, icon: Icon, emptyDescription }: GenreListPageProps) {
  const [search, setSearch] = useState('')
  const navigate = useNavigate()

  const { data, isLoading } = useTags({ key: 'genre', contentType })
  const filtered = filterTags(data?.data ?? [], search)

  return (
    <div>
      <PageHeader title="Genres" accent={accent} search={search} onSearch={setSearch} total={filtered.length} />
      <div className="px-8 py-6">
        {isLoading ? (
          <div className="flex flex-wrap gap-2">
            {Array.from({ length: 20 }).map((_, i) => (
              <div key={i} className="h-9 rounded-full bg-white/5 animate-pulse" style={{ width: `${70 + (i % 5) * 20}px` }} />
            ))}
          </div>
        ) : !filtered.length ? (
          <EmptyState icon={Icon} title="No genres yet" description={emptyDescription} accent={accent} />
        ) : (
          <div className="flex flex-wrap gap-2">
            {filtered.map(tag => (
              <button
                key={tag.id}
                type="button"
                onClick={() => navigate(toEntryHref(toSlug(tag.value)))}
                className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full text-sm font-medium border border-white/10 bg-white/5 text-white/70 hover:text-white hover:border-white/25 hover:bg-white/10 transition-all duration-150"
              >
                <Icon size={12} className="text-white/40" />
                {tag.value}
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
