import type { ReactNode } from 'react'

export interface HeroProps {
  backdropSrc?: string
  title: string
  // The facts row — short caller-supplied strings rendered inline,
  // separated by a middot (e.g. an Artist Detail's "4 albums · 1987–present",
  // an Album Detail's "2001 · Rock · 12 tracks"). Hero renders whatever
  // list it's given; it attaches no meaning to individual entries.
  facts?: string[]
  // Primary action slot (e.g. an Edit button) — rendered below the facts
  // row. Caller-supplied, same pattern as Card's badge slot.
  actions?: ReactNode
}

// Hero — style-guide Component vocabulary entry, #658. Large detail-view
// top section (backdrop image, title, facts row, action slot),
// config-driven per content type via props — see
// docs/design/style-guide.md#component-vocabulary and ADR 0004's
// shared-component rule (this file's test proves both an Artist Detail
// and an Album Detail configuration).
//
// The backdrop is decorative page furniture, not an interactive element
// — alt="" per WCAG 1.1.1, since the title/facts already carry the same
// information as text. No click/lightbox behavior of its own, same
// reasoning as Card (#658): if a caller ever wants the backdrop
// click-to-enlarge, that's composed from outside with ImageLightbox.
export function Hero({ backdropSrc, title, facts, actions }: HeroProps) {
  return (
    <div className="relative w-full overflow-hidden rounded-2xl border border-border">
      <div className="absolute inset-0" aria-hidden="true">
        {backdropSrc ? (
          <img src={backdropSrc} alt="" className="h-full w-full object-cover" />
        ) : (
          <div className="h-full w-full bg-surface-raised" />
        )}
        <div className="absolute inset-0 bg-gradient-to-t from-bg via-bg/70 to-transparent" />
      </div>

      <div className="relative flex aspect-[21/9] flex-col justify-end gap-4 p-6 sm:p-8">
        <h1 className="text-headline text-text">{title}</h1>

        {facts && facts.length > 0 && (
          <div className="flex flex-wrap items-center gap-2 text-body text-text-secondary">
            {facts.map((fact, index) => (
              <span key={fact} className="flex items-center gap-2">
                {index > 0 && (
                  <span aria-hidden="true" className="text-text-muted">
                    ·
                  </span>
                )}
                {fact}
              </span>
            ))}
          </div>
        )}

        {actions && <div className="flex flex-wrap items-center gap-3">{actions}</div>}
      </div>
    </div>
  )
}
