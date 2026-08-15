import { useState } from 'react'
import { X } from 'lucide-react'

// ScanRootValue mirrors internal/config/pipeline.go's ScanRoot
// ({path, content_type}) — the one struct-array config key
// (pipeline.scan_roots) in the DB-overlay schema. Exported so
// web/src/pages/settings/settingValue.ts's ParsedSettingValue can widen
// to include it without either file reaching into the other's page/pages
// boundary in the wrong direction (pages may import from components;
// components never import from pages).
export interface ScanRootValue {
  path: string
  content_type: string
}

export interface ScanRootsInputProps {
  label: string
  value: ScanRootValue[]
  onChange: (value: ScanRootValue[]) => void
  disabled?: boolean
}

// CONTENT_TYPE_OPTIONS mirrors internal/domain/enums.go's ContentType
// values.
const CONTENT_TYPE_OPTIONS = [
  { value: 'movie', label: 'Movies' },
  { value: 'tv', label: 'TV' },
  { value: 'music', label: 'Music' },
  { value: 'book', label: 'Books' },
  { value: 'adult', label: 'AfterDark' },
]

// ScanRootsInput is pipeline.scan_roots' dedicated editor — the generic
// ListInput assumes string[] and would render each row as
// "[object Object]" if handed this struct array directly. Rows are
// path + content-type pairs; caller order is preserved, matching every
// other list field in this codebase (no server- or client-side
// reordering of what's already there).
export function ScanRootsInput({ label, value, onChange, disabled }: ScanRootsInputProps) {
  const [draftPath, setDraftPath] = useState('')
  const [draftType, setDraftType] = useState(CONTENT_TYPE_OPTIONS[0].value)

  function add() {
    const trimmed = draftPath.trim()
    if (trimmed === '') return
    onChange([...value, { path: trimmed, content_type: draftType }])
    setDraftPath('')
  }

  function remove(index: number) {
    onChange(value.filter((_, i) => i !== index))
  }

  function updateRow(index: number, patch: Partial<ScanRootValue>) {
    onChange(value.map((row, i) => (i === index ? { ...row, ...patch } : row)))
  }

  return (
    <div className="flex flex-col gap-1.5 text-body text-text">
      <span className="text-label text-text-secondary">{label}</span>
      {value.length > 0 && (
        <ul className="flex flex-col gap-1.5">
          {value.map((row, index) => (
            <li key={index} className="flex items-center gap-2">
              <input
                type="text"
                aria-label={`${label} path ${index + 1}`}
                value={row.path}
                disabled={disabled}
                onChange={e => updateRow(index, { path: e.target.value })}
                className="h-9 flex-1 px-3 rounded-lg bg-surface-raised border border-border text-text text-body focus:outline-none focus:border-border-hover disabled:opacity-50"
              />
              <select
                aria-label={`${label} content type ${index + 1}`}
                value={row.content_type}
                disabled={disabled}
                onChange={e => updateRow(index, { content_type: e.target.value })}
                className="h-9 px-2 rounded-lg bg-surface-raised border border-border text-text text-body focus:outline-none focus:border-border-hover disabled:opacity-50"
              >
                {CONTENT_TYPE_OPTIONS.map(opt => (
                  <option key={opt.value} value={opt.value}>
                    {opt.label}
                  </option>
                ))}
              </select>
              {!disabled && (
                <button
                  type="button"
                  onClick={() => remove(index)}
                  aria-label={`Remove ${row.path || `row ${index + 1}`}`}
                  className="text-text-secondary hover:text-text"
                >
                  <X size={14} />
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
            placeholder="/path/to/watch"
            value={draftPath}
            onChange={e => setDraftPath(e.target.value)}
            onKeyDown={e => {
              if (e.key === 'Enter') {
                e.preventDefault()
                add()
              }
            }}
            className="h-9 flex-1 px-3 rounded-lg bg-surface-raised border border-border text-text text-body focus:outline-none focus:border-border-hover"
          />
          <select
            aria-label={`Content type for new ${label} entry`}
            value={draftType}
            onChange={e => setDraftType(e.target.value)}
            className="h-9 px-2 rounded-lg bg-surface-raised border border-border text-text text-body focus:outline-none focus:border-border-hover"
          >
            {CONTENT_TYPE_OPTIONS.map(opt => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
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
