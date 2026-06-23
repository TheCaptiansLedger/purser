import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Tag as TagIcon } from 'lucide-react'
import { useTags } from '../../api/tags'
import { PageHeader } from '../../components/layout/PageHeader'
import { EmptyState } from '../../components/ui/EmptyState'

const ACCENT = '#10b981'

export function LabelsPage() {
  const [search, setSearch] = useState('')
  const navigate = useNavigate()

  const { data, isLoading } = useTags({ key: 'label', contentType: 'music' })

  const filtered = (data?.data ?? []).filter(t =>
    !search || t.value.toLowerCase().includes(search.toLowerCase())
  )

  return (
    <div>
      <PageHeader
        title="Labels"
        accent={ACCENT}
        search={search}
        onSearch={setSearch}
        total={filtered.length}
      />
      <div className="px-8 py-6">
        {isLoading ? (
          <div className="flex flex-wrap gap-2">
            {Array.from({ length: 30 }).map((_, i) => (
              <div key={i} className="h-9 rounded-full bg-white/5 animate-pulse" style={{ width: `${80 + (i % 5) * 25}px` }} />
            ))}
          </div>
        ) : !filtered.length ? (
          <EmptyState icon={TagIcon} title="No labels yet" description="Tag albums with key=label to see them here." accent={ACCENT} />
        ) : (
          <div className="flex flex-wrap gap-2">
            {filtered.map(tag => (
              <button
                key={tag.id}
                type="button"
                onClick={() => navigate(`/music/labels/${encodeURIComponent(tag.value)}`)}
                className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full text-sm font-medium border border-white/10 bg-white/5 text-white/70 hover:text-white hover:border-white/25 hover:bg-white/10 transition-all duration-150"
                style={{ '--accent': ACCENT } as React.CSSProperties}
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
