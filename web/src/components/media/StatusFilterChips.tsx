import type { ItemStatus } from '../../types'
import { statusConfig } from './ItemStatusBadge'

export const STATUSES: ItemStatus[] = ['wanted', 'grabbed', 'downloading', 'imported', 'missing', 'skipped']

interface Props {
  value: ItemStatus | undefined
  onChange: (s: ItemStatus | undefined) => void
  accent?: string
}

export function StatusFilterChips({ value, onChange, accent }: Props) {
  return (
    <div className="flex items-center gap-1 flex-wrap">
      <button
        onClick={() => onChange(undefined)}
        className={[
          'text-xs px-2.5 py-1 rounded-lg border transition-colors',
          value === undefined
            ? 'border-transparent text-white'
            : 'border-white/10 text-white/40 hover:text-white/70 hover:border-white/20',
        ].join(' ')}
        style={value === undefined && accent ? { background: accent + '33', color: accent, borderColor: accent + '55' } : {}}
      >
        All
      </button>
      {STATUSES.map((s) => {
        const cfg = statusConfig(s)
        const active = value === s
        return (
          <button
            key={s}
            onClick={() => onChange(s)}
            className={[
              'text-xs px-2.5 py-1 rounded-lg border transition-colors',
              active ? 'border-transparent' : 'border-white/10 text-white/40 hover:text-white/70 hover:border-white/20',
            ].join(' ')}
            style={active ? { background: cfg.color + '22', color: cfg.color, borderColor: cfg.color + '44' } : {}}
          >
            {cfg.label}
          </button>
        )
      })}
    </div>
  )
}
