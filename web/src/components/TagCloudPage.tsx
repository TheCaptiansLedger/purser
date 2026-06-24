import { useState } from 'react'
import { Tag as TagIcon } from 'lucide-react'
import { useTags } from '../api/tags'
import { PageHeader } from './layout/PageHeader'
import { EmptyState } from './ui/EmptyState'
import type { Tag } from '../types'

interface TagCloudPageProps {
  contentType: string
  accent: string
  onTagClick?: (tag: Tag) => void
}

export function filterTags(tags: Tag[], search: string): Tag[] {
  if (!search) return tags
  const q = search.toLowerCase()
  return tags.filter(t => t.value.toLowerCase().includes(q))
}

export function TagCloudPage({ contentType, accent, onTagClick }: TagCloudPageProps) {
  const [search, setSearch] = useState('')
  const tags = useTags({ scope: 'metadata', contentType })
  const filtered = filterTags(tags.data?.data ?? [], search)

  return (
    <div>
      <PageHeader title="Tags" accent={accent} search={search} onSearch={setSearch} total={filtered.length} />
      <div className="px-8 py-6">
        {tags.isLoading ? (
          <div className="flex flex-wrap gap-2">
            {Array.from({ length: 40 }).map((_, i) => (
              <div key={i} className="h-8 rounded-full bg-white/5 animate-pulse" style={{ width: `${60 + (i % 5) * 20}px` }} />
            ))}
          </div>
        ) : !filtered.length ? (
          <EmptyState icon={TagIcon} title="No tags yet" accent={accent} />
        ) : (
          <div className="flex flex-wrap gap-2">
            {filtered.map(tag =>
              onTagClick ? (
                <button
                  key={tag.id}
                  onClick={() => onTagClick(tag)}
                  className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full text-sm font-medium border border-white/10 bg-white/5 text-white/70 hover:text-white hover:border-white/20 transition-all duration-150"
                >
                  <TagIcon size={12} className="text-white/40" />
                  {tag.value}
                </button>
              ) : (
                <span
                  key={tag.id}
                  className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full text-sm font-medium border border-white/10 bg-white/5 text-white/70 cursor-default select-none"
                >
                  <TagIcon size={12} className="text-white/40" />
                  {tag.value}
                </span>
              )
            )}
          </div>
        )}
      </div>
    </div>
  )
}
