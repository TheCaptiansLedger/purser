import { useState } from 'react'
import { Link } from 'react-router-dom'
import { Tag as TagIcon, Library } from 'lucide-react'
import { useTags } from '../../api/tags'
import { PageHeader } from '../../components/layout/PageHeader'
import { EmptyState } from '../../components/ui/EmptyState'

const ACCENT = '#10b981'

const KEY_LABELS: Record<string, string> = {
  label: 'Music Labels',
  genre: 'Genres',
}

export function getTagDisplayText(key: string, value: string): string {
  return key in KEY_LABELS ? value : `${key}:${value}`
}

export function getTagHref(key: string, value: string): string {
  if (key === 'label') return `/music/labels/${encodeURIComponent(value)}`
  return `/tags/${encodeURIComponent(key)}/${encodeURIComponent(value)}`
}

export function MusicTagsPage() {
  const [search, setSearch] = useState('')

  const { data, isLoading } = useTags({ contentType: 'music', limit: 500 })

  const allTags = (data?.data ?? []).filter(t =>
    !search || t.value.toLowerCase().includes(search.toLowerCase())
  )

  const grouped = allTags.reduce<Record<string, typeof allTags>>((acc, t) => {
    ;(acc[t.key] ??= []).push(t)
    return acc
  }, {})

  const keyOrder = ['label', 'genre', ...Object.keys(grouped).filter(k => k !== 'label' && k !== 'genre').sort()]
  const visibleKeys = keyOrder.filter(k => grouped[k]?.length)

  return (
    <div>
      <PageHeader
        title="Tags"
        accent={ACCENT}
        search={search}
        onSearch={setSearch}
        total={allTags.length}
      />
      <div className="px-8 py-6 space-y-8">
        {isLoading ? (
          <div className="flex flex-wrap gap-2">
            {Array.from({ length: 40 }).map((_, i) => (
              <div key={i} className="h-8 rounded-full bg-white/5 animate-pulse" style={{ width: `${60 + (i % 5) * 20}px` }} />
            ))}
          </div>
        ) : !allTags.length ? (
          <EmptyState icon={TagIcon} title="No tags yet" accent={ACCENT} />
        ) : (
          visibleKeys.map(key => {
            const tags = grouped[key]
            const isLabel = key === 'label'
            return (
              <section key={key}>
                <div className="flex items-center gap-2 mb-3">
                  {isLabel && <Library size={14} style={{ color: ACCENT }} />}
                  <h2 className="text-xs font-semibold uppercase tracking-widest text-white/40">
                    {KEY_LABELS[key] ?? key}
                  </h2>
                </div>
                <div className="flex flex-wrap gap-2">
                  {tags.map(tag => (
                    <Link
                      key={tag.id}
                      to={getTagHref(key, tag.value)}
                      className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full text-sm font-medium border border-white/10 bg-white/5 text-white/70 hover:text-white hover:border-white/20 hover:bg-white/8 transition-all duration-150"
                      style={isLabel ? { borderColor: ACCENT + '33' } : undefined}
                    >
                      {isLabel ? (
                        <Library size={11} style={{ color: ACCENT }} />
                      ) : (
                        <TagIcon size={11} className="text-white/40" />
                      )}
                      {getTagDisplayText(key, tag.value)}
                    </Link>
                  ))}
                </div>
              </section>
            )
          })
        )}
      </div>
    </div>
  )
}
