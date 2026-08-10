import { useState } from 'react'
import { X } from 'lucide-react'

export interface ListInputProps {
  label: string
  value: string[]
  onChange: (value: string[]) => void
  disabled?: boolean
}

// ListInput is the shared string-array field — e.g. a module's roots or
// afterdark.provider_priority. Caller order is preserved exactly: items
// only move via explicit remove-then-re-add, never sorted, per this
// project's "no server-side (or client-side) candidate/list reordering"
// rule.
export function ListInput({ label, value, onChange, disabled }: ListInputProps) {
  const [draft, setDraft] = useState('')

  function add() {
    const trimmed = draft.trim()
    if (trimmed === '') return
    onChange([...value, trimmed])
    setDraft('')
  }

  function remove(index: number) {
    onChange(value.filter((_, i) => i !== index))
  }

  return (
    <div className="flex flex-col gap-1.5 text-body text-text">
      <span className="text-label text-text-secondary">{label}</span>
      {value.length > 0 && (
        <ul className="flex flex-wrap gap-1.5">
          {value.map((item, index) => (
            <li
              key={`${item}-${index}`}
              className="flex items-center gap-1 h-7 px-2 rounded-md bg-surface-raised text-label text-text"
            >
              {item}
              {!disabled && (
                <button
                  type="button"
                  onClick={() => remove(index)}
                  aria-label={`Remove ${item}`}
                  className="text-text-secondary hover:text-text"
                >
                  <X size={12} />
                </button>
              )}
            </li>
          ))}
        </ul>
      )}
      {!disabled && (
        <div className="flex gap-2">
          <input
            type="text"
            aria-label={`Add to ${label}`}
            value={draft}
            onChange={e => setDraft(e.target.value)}
            onKeyDown={e => {
              if (e.key === 'Enter') {
                e.preventDefault()
                add()
              }
            }}
            className="h-9 flex-1 px-3 rounded-lg bg-surface-raised border border-border text-text text-body focus:outline-none focus:border-border-hover"
          />
          <button
            type="button"
            onClick={add}
            className="h-9 px-3 rounded-lg bg-surface-raised text-text-secondary hover:text-text text-body"
          >
            Add
          </button>
        </div>
      )}
    </div>
  )
}
