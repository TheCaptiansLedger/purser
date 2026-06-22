import { useState } from 'react'
import { Plus, X } from 'lucide-react'

interface AliasListProps {
  value: string[]
  onChange: (value: string[]) => void
  disabled?: boolean
}

export function canAddAlias(input: string, existing: string[]): boolean {
  const trimmed = input.trim()
  return trimmed.length > 0 && !existing.includes(trimmed)
}

export function AliasList({ value, onChange, disabled }: AliasListProps) {
  const [input, setInput] = useState('')

  const add = () => {
    if (!canAddAlias(input, value)) return
    onChange([...value, input.trim()])
    setInput('')
  }

  return (
    <div className="space-y-1.5">
      {value.map((alias, i) => (
        <div
          key={i}
          className="flex items-center gap-2 rounded-lg border border-white/10 bg-white/5 px-3 py-1.5"
        >
          <span className="flex-1 truncate text-sm text-white/70">{alias}</span>
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
      {!disabled && (
        <div className="flex gap-2">
          <input
            value={input}
            onChange={e => setInput(e.target.value)}
            onKeyDown={e => { if (e.key === 'Enter') { e.preventDefault(); add() } }}
            placeholder="Add alias…"
            className="flex-1 rounded-lg border border-white/10 bg-white/5 px-3 py-1.5 text-sm text-white placeholder:text-white/25 transition-colors focus:border-white/25 focus:outline-none"
          />
          <button
            type="button"
            onClick={add}
            className="rounded-lg border border-white/10 px-3 py-1.5 text-white/50 transition-colors hover:border-white/20 hover:text-white/80"
          >
            <Plus size={14} />
          </button>
        </div>
      )}
    </div>
  )
}
