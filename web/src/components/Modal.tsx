import { useEffect, useRef, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { X } from 'lucide-react'

export interface ModalProps {
  title: string
  onClose: () => void
  children: ReactNode
}

// Modal is the first modal/dialog primitive in this codebase — the shell
// docs/design/ux-principles.md#navigation--information-architecture calls
// for ("modal drill-down for editing, route-based navigation for
// browsing"). It owns dialog chrome only (overlay, focus, Esc/backdrop
// dismissal) — the job detail modal (#608) is the first consumer, but any
// later edit dialog builds on this rather than rolling its own, per ADR
// 0002's SRP/OCP guidance.
//
// The caller owns visibility: Modal always renders once mounted, so
// showing/hiding it is "mount JobDetailModal or don't" rather than an
// `open` prop here — one fewer state to keep in sync.
export function Modal({ title, onClose, children }: ModalProps) {
  const dialogRef = useRef<HTMLDivElement>(null)

  // Move focus into the dialog on mount (WCAG 2.4.3 focus order) and close
  // on Escape (WCAG 2.1.2 no keyboard trap — Esc is the documented way out).
  useEffect(() => {
    dialogRef.current?.focus()

    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape') {
        onClose()
      }
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [onClose])

  return createPortal(
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-bg/70 p-4" onClick={onClose}>
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-label={title}
        tabIndex={-1}
        onClick={event => event.stopPropagation()}
        className="flex max-h-[85vh] w-full max-w-2xl flex-col gap-4 rounded-xl bg-surface-raised p-6 shadow-xl focus:outline-none"
      >
        <div className="flex items-center justify-between gap-4">
          <h2 className="text-title-md text-text">{title}</h2>
          <button
            type="button"
            onClick={onClose}
            aria-label="Close"
            className="rounded-lg p-1 text-text-secondary hover:bg-surface hover:text-text"
          >
            <X size={18} />
          </button>
        </div>
        <div className="overflow-y-auto">{children}</div>
      </div>
    </div>,
    document.body,
  )
}
