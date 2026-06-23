import { useState, useRef, useEffect } from 'react'
import { X, Plus, Tag as TagIcon } from 'lucide-react'
import { useTags, useCreateTag } from '../../../api/tags'
import type { Tag } from '../../../types'

interface TagPickerProps {
  value: Tag[]
  onAdd?: (tag: Tag) => void
  onRemove?: (tagId: string) => void
  disabled?: boolean
  hideKeys?: string[]
}

export function TagPicker({ value, onAdd, onRemove, disabled, hideKeys }: TagPickerProps) {
  const [keyInput, setKeyInput] = useState('')
  const [valueInput, setValueInput] = useState('')
  const [keyOpen, setKeyOpen] = useState(false)
  const [valueOpen, setValueOpen] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)

  const { data: allTagsPage } = useTags({ limit: 500 })
  const { data: keyTagsPage } = useTags({ key: keyInput.trim() || undefined, limit: 500 })
  const createTag = useCreateTag()

  const existingIds = new Set(value.map(t => t.id))

  const allTags = allTagsPage?.data ?? []
  const allKeys = [...new Set(allTags.map(t => t.key))].sort()
  const keySuggestions = allKeys.filter(k => !keyInput || k.toLowerCase().includes(keyInput.toLowerCase()))

  const keyTags = keyTagsPage?.data ?? []
  const valueSuggestions = keyTags.filter(t =>
    !existingIds.has(t.id) && (!valueInput || t.value.toLowerCase().includes(valueInput.toLowerCase()))
  )

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setKeyOpen(false)
        setValueOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClick)
    return () => document.removeEventListener('mousedown', handleClick)
  }, [])

  async function handleAdd() {
    const key = keyInput.trim()
    const val = valueInput.trim()
    if (!key || !val || !onAdd) return

    const existing = keyTags.find(
      t => t.key.toLowerCase() === key.toLowerCase() && t.value.toLowerCase() === val.toLowerCase()
    )
    if (existing) {
      if (!existingIds.has(existing.id)) onAdd(existing)
    } else {
      const newTag = await createTag.mutateAsync({ key, value: val, scope: 'user' })
      onAdd(newTag)
    }
    setKeyInput('')
    setValueInput('')
  }

  return (
    <div ref={containerRef} className="space-y-3">
      <div className="flex min-h-[38px] flex-wrap gap-1.5 rounded-lg border border-white/10 bg-white/5 px-3 py-2">
        {value.length > 0 ? value.map(tag => (
          <span
            key={tag.id}
            className="inline-flex items-center gap-1 rounded-full bg-white/10 px-2 py-0.5 text-xs text-white/70"
          >
            {!hideKeys?.includes(tag.key) && (
              <span className="font-mono text-white/40">{tag.key}:</span>
            )}
            <span>{tag.value}</span>
            {!disabled && onRemove && (
              <button
                type="button"
                onClick={() => onRemove(tag.id)}
                className="text-white/40 transition-colors hover:text-white/80"
              >
                <X size={10} />
              </button>
            )}
          </span>
        )) : (
          <span className="text-sm text-white/25">No tags</span>
        )}
      </div>

      {!disabled && onAdd && (
        <div className="flex items-center gap-2">
          <div className="relative flex-1">
            <div className="flex items-center gap-1.5 rounded-lg border border-white/10 bg-white/5 px-3 py-1.5">
              <TagIcon size={12} className="shrink-0 text-white/30" />
              <input
                value={keyInput}
                onChange={e => { setKeyInput(e.target.value); setKeyOpen(true); setValueOpen(false) }}
                onFocus={() => setKeyOpen(true)}
                placeholder="key"
                className="min-w-0 flex-1 bg-transparent text-sm text-white/70 placeholder-white/25 outline-none"
              />
            </div>
            {keyOpen && keySuggestions.length > 0 && (
              <div className="absolute z-50 mt-1 max-h-48 w-full overflow-y-auto rounded-lg border border-white/10 bg-zinc-900 shadow-xl">
                {keySuggestions.map(k => (
                  <button
                    key={k}
                    type="button"
                    onMouseDown={e => { e.preventDefault(); setKeyInput(k); setKeyOpen(false) }}
                    className="flex w-full items-center px-3 py-2 text-sm text-white/70 transition-colors hover:bg-white/8 hover:text-white"
                  >
                    {k}
                  </button>
                ))}
              </div>
            )}
          </div>

          <span className="shrink-0 text-sm text-white/30">:</span>

          <div className="relative flex-1">
            <input
              value={valueInput}
              onChange={e => { setValueInput(e.target.value); setValueOpen(true); setKeyOpen(false) }}
              onFocus={() => setValueOpen(true)}
              onKeyDown={e => { if (e.key === 'Enter') { e.preventDefault(); void handleAdd() } }}
              placeholder="value"
              className="w-full rounded-lg border border-white/10 bg-white/5 px-3 py-1.5 text-sm text-white/70 placeholder-white/25 outline-none"
            />
            {valueOpen && valueSuggestions.length > 0 && (
              <div className="absolute z-50 mt-1 max-h-48 w-full overflow-y-auto rounded-lg border border-white/10 bg-zinc-900 shadow-xl">
                {valueSuggestions.map(t => (
                  <button
                    key={t.id}
                    type="button"
                    onMouseDown={e => { e.preventDefault(); setValueInput(t.value); setValueOpen(false) }}
                    className="flex w-full items-center px-3 py-2 text-sm text-white/70 transition-colors hover:bg-white/8 hover:text-white"
                  >
                    {t.value}
                  </button>
                ))}
              </div>
            )}
          </div>

          <button
            type="button"
            onClick={() => void handleAdd()}
            disabled={!keyInput.trim() || !valueInput.trim() || createTag.isPending}
            className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-white/10 bg-white/5 text-white/50 transition-all hover:border-white/25 hover:bg-white/10 hover:text-white disabled:cursor-not-allowed disabled:opacity-30"
          >
            <Plus size={14} />
          </button>
        </div>
      )}
    </div>
  )
}
