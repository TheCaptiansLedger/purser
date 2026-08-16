import { useEffect, useRef } from 'react'
import { createPortal } from 'react-dom'
import { X } from 'lucide-react'

export interface ImageLightboxProps {
  src: string
  alt: string
  onClose: () => void
}

const FOCUSABLE_SELECTOR =
  'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'

// ImageLightbox — style-guide Component vocabulary entry, pre-reset
// precedent #288. Full-viewport, content-type agnostic image viewer: it
// takes a plain src/alt and reuses the bytes the caller already loaded
// (GET /media/images/{id}, #651) rather than fetching a hi-res copy.
//
// Distinct from Modal (this codebase's other dismissable-overlay
// primitive): Modal renders titled dialog chrome for editing flows, this
// renders a full-bleed image with no chrome beyond a close button, so it
// isn't built on top of Modal. It does implement a real focus trap and
// restores focus to the trigger element on close (WCAG 2.2 baseline per
// the style guide) — Modal only moves focus in on open today.
export function ImageLightbox({ src, alt, onClose }: ImageLightboxProps) {
  const dialogRef = useRef<HTMLDivElement>(null)
  const closeButtonRef = useRef<HTMLButtonElement>(null)
  const triggerRef = useRef<Element | null>(null)

  // Capture the trigger on mount, move focus into the overlay, and restore
  // focus to the trigger on unmount (WCAG 2.4.3 / focus restoration).
  useEffect(() => {
    triggerRef.current = document.activeElement
    closeButtonRef.current?.focus()

    return () => {
      if (triggerRef.current instanceof HTMLElement) {
        triggerRef.current.focus()
      }
    }
  }, [])

  // Lock body scroll while the overlay is open.
  useEffect(() => {
    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      document.body.style.overflow = previousOverflow
    }
  }, [])

  // Escape closes; Tab/Shift+Tab is trapped inside the overlay's focusable
  // elements (WCAG 2.1.2 — the trap has a documented way out via Escape).
  useEffect(() => {
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape') {
        onClose()
        return
      }
      if (event.key !== 'Tab') return

      const dialog = dialogRef.current
      if (!dialog) return
      const focusable = Array.from(dialog.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR))
      if (focusable.length === 0) return

      const first = focusable[0]
      const last = focusable[focusable.length - 1]

      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault()
        last.focus()
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault()
        first.focus()
      }
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [onClose])

  return createPortal(
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/90 p-4" onClick={onClose}>
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-label={alt}
        className="relative max-h-full max-w-full"
        onClick={event => event.stopPropagation()}
      >
        <button
          ref={closeButtonRef}
          type="button"
          onClick={onClose}
          aria-label="Close"
          className="absolute -top-10 right-0 p-2 text-white/70 transition-colors hover:text-white"
        >
          <X size={24} />
        </button>
        <img src={src} alt={alt} className="max-h-[90vh] max-w-full object-contain" />
      </div>
    </div>,
    document.body,
  )
}
