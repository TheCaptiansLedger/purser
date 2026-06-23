import { X } from 'lucide-react'
import type { Tag } from '../../../types'

interface TagPickerProps {
  value: Tag[]
  onChange: (value: Tag[]) => void
  disabled?: boolean
}

export function TagPicker({ value, onChange, disabled }: TagPickerProps) {
  return (
    <div className="flex min-h-[38px] flex-wrap gap-1.5 rounded-lg border border-white/10 bg-white/5 px-3 py-2">
      {value.map(tag => (
        <span
          key={tag.id}
          className="inline-flex items-center gap-1 rounded-full bg-white/10 px-2 py-0.5 text-xs text-white/70"
        >
          {tag.value}
          {!disabled && (
            <button
              type="button"
              onClick={() => onChange(value.filter(t => t.id !== tag.id))}
              className="text-white/40 transition-colors hover:text-white/80"
            >
              <X size={10} />
            </button>
          )}
        </span>
      ))}
      {value.length === 0 && (
        <span className="text-sm text-white/25">No tags</span>
      )}
    </div>
  )
}
