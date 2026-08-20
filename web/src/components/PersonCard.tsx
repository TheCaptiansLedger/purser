import { Pencil, Trash2, User } from 'lucide-react'
import { useState } from 'react'
import type { PersonRef } from '../types'
import { ImageLightbox } from './ImageLightbox'

// PersonCardRole is one role chip. `label` is the only field the People
// index config needs (plain display text, already formatted with any era
// suffix); `id` and the action callbacks are #724's Artist Detail Members
// tab addition — a chip with either callback grows small inline Edit/
// Remove icons, keyed/labeled by `id` (the raw EntryPerson `role` string,
// distinct from `label`'s formatted text) rather than `label` itself,
// since two chips could otherwise share a label.
export interface PersonCardRole {
  label: string
  id?: string
  onEdit?: () => void
  onRemove?: () => void
}

export interface PersonCardProps {
  person: PersonRef
  // Role chip(s) — plural because EntryPerson's (person_id, role) key
  // means one person can carry more than one role on the same entry.
  // Omitted entirely by the People index config; the Artist Detail
  // Members tab config passes one or more. PersonCard renders whatever
  // list it's given — no Music/People-specific meaning attached here.
  roles?: PersonCardRole[]
}

// PersonCard — style-guide Component vocabulary entry, #657. Shared
// between the People index page and the Artist Detail Members tab, reused
// unmodified in shape (only extended, never forked) per ADR 0002's
// SRP/OCP. Accepts a plain PersonRef, never the full Person interface —
// see PersonRef's own comment in web/src/types/index.ts for why
// (pre-reset issue #225).
//
// Not built on the not-yet-existing Card/Hero primitives (#658) — same
// self-contained precedent ImageLightbox set relative to Modal.
//
// Deliberately has no click-through/navigation prop: no Person detail
// route exists yet (#661, not built), and the photo already owns one
// click target (opens ImageLightbox). A caller that needs the whole card
// to navigate wraps the rendered PersonCard from outside.
export function PersonCard({ person, roles }: PersonCardProps) {
  const [lightboxOpen, setLightboxOpen] = useState(false)
  const imageSrc = person.imageId ? `/media/images/${person.imageId}` : undefined

  return (
    <div className="flex flex-col items-center gap-2 p-4 text-center">
      {imageSrc ? (
        <button
          type="button"
          onClick={() => setLightboxOpen(true)}
          aria-label={`View ${person.name}'s photo`}
          className="aspect-square w-full overflow-hidden rounded-full border border-border"
        >
          <img src={imageSrc} alt={person.name} className="h-full w-full object-cover" />
        </button>
      ) : (
        <div
          aria-hidden="true"
          className="flex aspect-square w-full items-center justify-center rounded-full border border-border bg-surface-raised text-text-secondary"
        >
          <User size={28} />
        </div>
      )}

      <span className="text-body font-medium text-text">{person.name}</span>

      {roles && roles.length > 0 && (
        <div className="flex flex-wrap items-center justify-center gap-1">
          {roles.map(role => {
            const rowKey = role.id ?? role.label
            return (
              <span
                key={rowKey}
                className="inline-flex items-center gap-1 h-5 pl-1.5 pr-1 rounded-sm bg-surface-raised text-label text-text-secondary"
              >
                {role.label}
                {role.onEdit && (
                  <button
                    type="button"
                    onClick={role.onEdit}
                    aria-label={`Edit ${person.name}'s ${rowKey} role`}
                    title="Edit role"
                    className="flex h-3.5 w-3.5 items-center justify-center rounded hover:text-text"
                  >
                    <Pencil size={10} />
                  </button>
                )}
                {role.onRemove && (
                  <button
                    type="button"
                    onClick={role.onRemove}
                    aria-label={`Remove ${person.name}'s ${rowKey} role`}
                    title="Remove"
                    className="flex h-3.5 w-3.5 items-center justify-center rounded hover:text-status-failure"
                  >
                    <Trash2 size={10} />
                  </button>
                )}
              </span>
            )
          })}
        </div>
      )}

      {lightboxOpen && (
        <ImageLightbox src={imageSrc!} alt={person.name} onClose={() => setLightboxOpen(false)} />
      )}
    </div>
  )
}
