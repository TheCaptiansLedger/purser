import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Clapperboard } from 'lucide-react'
import { useTags } from '../../api/tags'
import { PageHeader } from '../../components/layout/PageHeader'
import { EmptyState } from '../../components/ui/EmptyState'

const ACCENT = '#8b5cf6'

export function TVGenresPage() {
  const [search, setSearch] = useState('')
  const navigate = useNavigate()

  const { data, isLoading } = useTags({ key: 'genre', contentType: 'tv' })

  const filtered = (data?.data ?? []).filter(t =>
    !search || t.value.toLowerCase().includes(search.toLowerCase())
  )

  function toSlug(value: string) {
    return value.toLowerCase().replace(/\s+/g, '-').replace(/[^a-z0-9-]/g, '')
  }

  return (
    <div>
      <PageHeader
        title="Genres"
        accent={ACCENT}
        search={search}
        onSearch={setSearch}
        total={filtered.length}
      />
      <div className="px-8 py-6">
        {isLoading ? (
          <div className="flex flex-wrap gap-2">
            {Array.from({ length: 20 }).map((_, i) => (
              <div key={i} className="h-9 rounded-full bg-white/5 animate-pulse" style={{ width: `${70 + (i % 5) * 20}px` }} />
            ))}
          </div>
        ) : !filtered.length ? (
          <EmptyState icon={Clapperboard} title="No genres yet" description="Tag TV series with key=genre to see them here." accent={ACCENT} />
        ) : (
          <div className="flex flex-wrap gap-2">
            {filtered.map(tag => (
              <button
                key={tag.id}
                type="button"
                onClick={() => navigate(`/tv/genre/${toSlug(tag.value)}`)}
                className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full text-sm font-medium border border-white/10 bg-white/5 text-white/70 hover:text-white hover:border-white/25 hover:bg-white/10 transition-all duration-150"
              >
                <Clapperboard size={12} className="text-white/40" />
                {tag.value}
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
