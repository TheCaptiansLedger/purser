import { Eye, EyeOff } from 'lucide-react'
import { SearchBar } from '../ui/SearchBar'

interface StatusOverlayControl {
  value: boolean
  onToggle: () => void
}

interface Props {
  title: string
  subtitle?: string
  accent: string
  search: string
  onSearch: (v: string) => void
  total?: number
  statusOverlay?: StatusOverlayControl
  children?: React.ReactNode
}

export function PageHeader({ title, subtitle, accent, search, onSearch, total, statusOverlay, children }: Props) {
  return (
    <div
      className="sticky top-0 z-30 border-b border-white/5 backdrop-blur-xl"
      style={{ background: 'rgba(8,8,14,0.85)' }}
    >
      <div className="px-6 py-4 flex items-center gap-3 flex-wrap">
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
        <div className="w-40 sm:w-52 md:w-64 shrink-0">
          <SearchBar value={search} onChange={onSearch} accent={accent} />
        </div>
        {statusOverlay && (
          <button
            type="button"
            onClick={statusOverlay.onToggle}
            title={statusOverlay.value ? 'Hide status badges' : 'Always show status badges'}
            className="inline-flex items-center gap-1.5 text-xs px-2.5 py-1.5 rounded-lg border transition-colors"
            style={statusOverlay.value
              ? { borderColor: accent + '55', color: accent }
              : { borderColor: 'rgba(255,255,255,0.1)', color: 'rgba(255,255,255,0.4)' }
            }
          >
            {statusOverlay.value ? <Eye size={13} /> : <EyeOff size={13} />}
            Status
          </button>
        )}
        {children}
      </div>
    </div>
  )
}
