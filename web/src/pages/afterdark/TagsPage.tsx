import { useState } from 'react'
import { Tag as TagIcon } from 'lucide-react'
import { useTags } from '../../api/tags'
import { PageHeader } from '../../components/layout/PageHeader'
import { EmptyState } from '../../components/ui/EmptyState'

const ACCENT = '#f43f5e'

export function TagsPage() {
  const [search, setSearch] = useState('')

  const tags = useTags({ scope: 'metadata', contentType: 'adult,jav' })

  const filtered = tags.data?.data.filter(t =>
    !search || t.name.toLowerCase().includes(search.toLowerCase())
  ) ?? []

  return (
    <div>
      <PageHeader
        title="Tags"
        accent={ACCENT}
        search={search}
        onSearch={setSearch}
        total={filtered.length}
      />
      <div className="px-8 py-6">
        {tags.isLoading ? (
          <div className="flex flex-wrap gap-2">
            {Array.from({ length: 40 }).map((_, i) => (
              <div key={i} className="h-8 rounded-full bg-white/5 animate-pulse" style={{ width: `${60 + (i % 5) * 20}px` }} />
            ))}
          </div>
        ) : !filtered.length ? (
          <EmptyState icon={TagIcon} title="No tags yet" accent={ACCENT} />
        ) : (
          <div className="flex flex-wrap gap-2">
            {filtered.map(tag => (
              <span
                key={tag.id}
                className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full text-sm font-medium border border-white/10 bg-white/5 text-white/70 hover:text-white hover:border-white/20 hover:bg-white/8 transition-all duration-150 cursor-default select-none"
                style={{ '--hover-color': ACCENT } as React.CSSProperties}
              >
                <TagIcon size={12} className="text-white/40" />
                {tag.name}
              </span>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
