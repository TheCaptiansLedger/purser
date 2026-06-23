import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Tag as TagIcon } from 'lucide-react'
import { useTags } from '../../api/tags'
import { PageHeader } from '../../components/layout/PageHeader'
import { EmptyState } from '../../components/ui/EmptyState'

const ACCENT = '#3b82f6'

export function MovieTagsPage() {
  const [search, setSearch] = useState('')
  const navigate = useNavigate()
  const tags = useTags({ scope: 'metadata', contentType: 'movie' })

  const filtered = (tags.data?.data ?? []).filter(t =>
    !search || t.value.toLowerCase().includes(search.toLowerCase())
  )

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
              <button
                key={tag.id}
                onClick={() => navigate(`/movies/genre/${encodeURIComponent(tag.value.toLowerCase().replace(/\s+/g, '-'))}`)}
                className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full text-sm font-medium border border-white/10 bg-white/5 text-white/70 hover:text-white hover:border-white/20 transition-all duration-150"
                style={{ ['--accent' as string]: ACCENT }}
              >
                <TagIcon size={12} className="text-white/40" />
                {tag.value}
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
