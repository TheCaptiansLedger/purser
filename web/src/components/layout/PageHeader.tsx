import { SearchBar } from '../ui/SearchBar'

interface Props {
  title: string
  subtitle?: string
  accent: string
  search: string
  onSearch: (v: string) => void
  total?: number
  children?: React.ReactNode
}

export function PageHeader({ title, subtitle, accent, search, onSearch, total, children }: Props) {
  return (
    <div
      className="sticky top-0 z-30 border-b border-white/5 backdrop-blur-xl"
      style={{ background: 'rgba(8,8,14,0.85)' }}
    >
      <div className="px-8 py-5 flex items-center gap-4">
        <div className="flex-1 min-w-0">
          <div className="flex items-baseline gap-3">
            <h1
              className="text-xl font-semibold"
              style={{ color: accent }}
            >
              {title}
            </h1>
            {total !== undefined && (
              <span className="text-sm text-white/30">{total.toLocaleString()}</span>
            )}
          </div>
          {subtitle && <p className="text-xs text-white/35 mt-0.5">{subtitle}</p>}
        </div>
        <div className="w-64 shrink-0">
          <SearchBar value={search} onChange={onSearch} accent={accent} />
        </div>
        {children}
      </div>
    </div>
  )
}
