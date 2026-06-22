import { X } from 'lucide-react'
import type { ExternalID } from '../../../types'

interface ExternalIDListProps {
  value: ExternalID[]
  onChange: (value: ExternalID[]) => void
  disabled?: boolean
}

export function ExternalIDList({ value, onChange, disabled }: ExternalIDListProps) {
  return (
    <div className="space-y-1.5">
      {value.map((eid, i) => (
        <div
          key={i}
          className="flex items-center gap-2 rounded-lg border border-white/10 bg-white/5 px-3 py-1.5"
        >
          <span className="w-24 shrink-0 font-mono text-xs text-white/40">{eid.source}</span>
          <span className="flex-1 truncate font-mono text-sm text-white/70">{eid.value}</span>
          {!disabled && (
            <button
              type="button"
              onClick={() => onChange(value.filter((_, j) => j !== i))}
              className="shrink-0 text-white/25 transition-colors hover:text-white/60"
            >
              <X size={13} />
            </button>
          )}
        </div>
      ))}
      {value.length === 0 && (
        <p className="px-3 py-2 text-sm text-white/25">No external IDs</p>
      )}
    </div>
  )
}
