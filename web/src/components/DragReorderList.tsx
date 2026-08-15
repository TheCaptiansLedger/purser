import { useState, type KeyboardEvent, type ReactNode } from 'react'
import { GripVertical } from 'lucide-react'

export interface DragReorderListProps<T> {
  items: T[]
  getKey: (item: T) => string
  // getLabel drives each row's accessible name (defaults to getKey) —
  // pass this whenever getKey returns a raw id that isn't itself
  // human-readable (e.g. a provider slug like "stashdb").
  getLabel?: (item: T) => string
  renderItem: (item: T) => ReactNode
  onReorder: (items: T[]) => void
  ariaLabel: string
  disabled?: boolean
}

// DragReorderList is a generic, reorder-only list (no add/remove — rank
// order is the only thing it edits) built on native HTML5 drag-and-drop
// for pointer users. Native drag-and-drop has no keyboard equivalent
// (WCAG 2.1.1), so every row's handle also answers ArrowUp/ArrowDown
// directly — same onReorder callback, two input paths, one behind the
// other never leaves the other stranded.
//
// First consumer is PriorityListInput (afterdark.provider_priority);
// generic over T so a future reorderable list doesn't need its own drag
// implementation from scratch.
export function DragReorderList<T>({
  items,
  getKey,
  getLabel = getKey,
  renderItem,
  onReorder,
  ariaLabel,
  disabled,
}: DragReorderListProps<T>) {
  const [dragIndex, setDragIndex] = useState<number | null>(null)

  function move(from: number, to: number) {
    if (disabled || to < 0 || to >= items.length || from === to) return
    const next = [...items]
    const [moved] = next.splice(from, 1)
    next.splice(to, 0, moved)
    onReorder(next)
  }

  function handleKeyDown(event: KeyboardEvent<HTMLButtonElement>, index: number) {
    if (event.key === 'ArrowUp') {
      event.preventDefault()
      move(index, index - 1)
    } else if (event.key === 'ArrowDown') {
      event.preventDefault()
      move(index, index + 1)
    }
  }

  return (
    <ul aria-label={ariaLabel} className="flex flex-col gap-1.5">
      {items.map((item, index) => (
        <li
          key={getKey(item)}
          draggable={!disabled}
          onDragStart={() => setDragIndex(index)}
          onDragOver={event => event.preventDefault()}
          onDrop={event => {
            event.preventDefault()
            if (dragIndex !== null) move(dragIndex, index)
            setDragIndex(null)
          }}
          onDragEnd={() => setDragIndex(null)}
          className="flex items-center gap-2 h-9 px-2 rounded-lg bg-surface-raised border border-border text-body text-text"
        >
          <button
            type="button"
            aria-label={`Reorder ${getLabel(item)}, position ${index + 1} of ${items.length}`}
            disabled={disabled}
            onKeyDown={event => handleKeyDown(event, index)}
            className="cursor-grab text-text-secondary hover:text-text disabled:cursor-not-allowed disabled:opacity-50 active:cursor-grabbing"
          >
            <GripVertical size={14} />
          </button>
          <span className="w-4 shrink-0 text-label text-text-secondary">{index + 1}</span>
          <span className="flex-1">{renderItem(item)}</span>
        </li>
      ))}
    </ul>
  )
}
