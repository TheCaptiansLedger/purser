import { useState } from 'react'
import { X, Plus } from 'lucide-react'
import type { ExternalID } from '../../../types'

export function canAddExternalID(source: string, value: string): boolean {
  return source.trim().length > 0 && value.trim().length > 0
}

interface ExternalIDListProps {
  value: ExternalID[]
  onChange: (value: ExternalID[]) => void
  disabled?: boolean
}

export function ExternalIDList({ value, onChange, disabled }: ExternalIDListProps) {
  const [source, setSource] = useState('')
  const [val, setVal] = useState('')

  function handleAdd() {
    if (!canAddExternalID(source, val)) return
    onChange([...value, { source: source.trim(), value: val.trim() }])
    setSource('')
    setVal('')
  }

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
      {!disabled && (
        <div className="flex items-center gap-2 pt-1">
          <input
            value={source}
            onChange={e => setSource(e.target.value)}
            placeholder="source"
            className="w-28 rounded-lg border border-white/10 bg-white/5 px-3 py-1.5 font-mono text-xs text-white/70 placeholder-white/25 outline-none focus:border-white/20"
          />
          <input
            value={val}
            onChange={e => setVal(e.target.value)}
            onKeyDown={e => { if (e.key === 'Enter') { e.preventDefault(); handleAdd() } }}
            placeholder="value"
            className="flex-1 rounded-lg border border-white/10 bg-white/5 px-3 py-1.5 font-mono text-sm text-white/70 placeholder-white/25 outline-none focus:border-white/20"
          />
          <button
            type="button"
            onClick={handleAdd}
            disabled={!canAddExternalID(source, val)}
            className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-white/10 bg-white/5 text-white/50 transition-all hover:border-white/25 hover:bg-white/10 hover:text-white disabled:cursor-not-allowed disabled:opacity-30"
          >
            <Plus size={14} />
          </button>
        </div>
      )}
    </div>
  )
}
