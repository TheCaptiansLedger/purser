import { useEffect, useRef, useState, type ReactNode } from 'react'

export interface DropdownMenuItem {
  label: string
  onSelect: () => void
  disabled?: boolean
}

export interface DropdownMenuProps {
  label: string
  items: DropdownMenuItem[]
  // trigger renders the button's own content (icon + label) — the caller
  // owns styling so this primitive stays free of any one call site's
  // button chroma, same "chrome only" split Modal draws with its content.
  trigger: ReactNode
  triggerClassName?: string
}

// DropdownMenu is this codebase's first trigger/menu primitive — a small,
// generic "pick one of a few actions" popover (WAI-ARIA menu button
// pattern: trigger owns `aria-haspopup`/`aria-expanded`, the popover is
// `role="menu"` of `role="menuitem"` buttons). Closes on outside click,
// Escape, or an item being selected. Deliberately minimal — no submenus,
// no keyboard arrow-key roving (not needed by its first consumer, Add
// Album's source picker, #669) — extend it if a second consumer needs
// more, per ADR 0002's YAGNI-leaning OCP.
export function DropdownMenu({ label, items, trigger, triggerClassName }: DropdownMenuProps) {
  const [open, setOpen] = useState(false)
  const rootRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return

    function handlePointerDown(event: MouseEvent) {
      if (rootRef.current && !rootRef.current.contains(event.target as Node)) {
        setOpen(false)
      }
    }
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape') setOpen(false)
    }
    document.addEventListener('mousedown', handlePointerDown)
    document.addEventListener('keydown', handleKeyDown)
    return () => {
      document.removeEventListener('mousedown', handlePointerDown)
      document.removeEventListener('keydown', handleKeyDown)
    }
  }, [open])

  return (
    <div ref={rootRef} className="relative inline-block">
      <button
        type="button"
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label={label}
        onClick={() => setOpen(prev => !prev)}
        className={triggerClassName}
      >
        {trigger}
      </button>

      {open && (
        <div
          role="menu"
          aria-label={label}
          className="absolute right-0 z-10 mt-1 flex min-w-40 flex-col gap-0.5 rounded-lg border border-border bg-surface-raised p-1 shadow-xl"
        >
          {items.map(item => (
            <button
              key={item.label}
              type="button"
              role="menuitem"
              disabled={item.disabled}
              onClick={() => {
                setOpen(false)
                item.onSelect()
              }}
              className="flex h-8 items-center rounded-md px-2 text-left text-body text-text hover:bg-surface disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {item.label}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
