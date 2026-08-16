import { User } from 'lucide-react'
import { useState } from 'react'
import type { PersonRef } from '../types'
import { ImageLightbox } from './ImageLightbox'

export interface PersonCardProps {
  person: PersonRef
  // Role chip(s) — plural because EntryPerson's (person_id, role) key
  // means one person can carry more than one role on the same entry.
  // Omitted entirely by the People index config; the Artist Detail
  // Members tab config passes one or more. PersonCard renders whatever
  // list it's given — no Music/People-specific meaning attached here.
  roles?: string[]
}

// PersonCard — style-guide Component vocabulary entry, #657. Shared
// between the People index page and the Artist Detail Members tab
// (neither built yet); reused unmodified per ADR 0002's SRP/OCP. Accepts
// a plain PersonRef, never the full Person interface — see PersonRef's
// own comment in web/src/types/index.ts for why (pre-reset issue #225).
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
          {roles.map(role => (
            <span
              key={role}
              className="inline-flex items-center h-5 px-1.5 rounded-sm bg-surface-raised text-label text-text-secondary"
            >
              {role}
            </span>
          ))}
        </div>
      )}

      {lightboxOpen && (
        <ImageLightbox src={imageSrc!} alt={person.name} onClose={() => setLightboxOpen(false)} />
      )}
    </div>
  )
}
