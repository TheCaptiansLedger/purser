import type { LucideIcon } from 'lucide-react'
import { ImageIcon } from 'lucide-react'
import type { ReactNode } from 'react'

export interface CardProps {
  imageSrc?: string
  title: string
  // Secondary line under the title — e.g. an AlbumCard's artist name, an
  // ArtistCard's genre. Purely caller-supplied text, no meaning attached
  // here.
  subtitle?: string
  // Status-badge slot (style-guide Component vocabulary entry, #658) —
  // rendered as an overlay on the artwork's corner. Callers pass their
  // own status element (e.g. a future StatusBadge); Card never branches
  // on what's inside it.
  badge?: ReactNode
  // Shown in place of artwork when imageSrc is unset. Defaults to a
  // generic icon so Card never imports a content-type-specific one
  // (Music/AfterDark) — callers pass their own (e.g. lucide's `Music`
  // for an ArtistCard) at the call site instead.
  placeholderIcon?: LucideIcon
}

// Card — style-guide Component vocabulary entry, #658. Poster/cover
// image + title + a status-badge slot, config-driven per content type
// via props — see docs/design/style-guide.md#component-vocabulary and
// ADR 0004's shared-component rule (this file's test proves both an
// ArtistCard and an AlbumCard configuration).
//
// Deliberately has no click/navigation/lightbox behavior of its own —
// same precedent PersonCard set (#657): a Music Library grid's
// ArtistCard needs the whole tile to navigate to Artist Detail, while an
// AlbumCard in the Discography tab might instead want click-to-enlarge.
// Those are different interactions on the same component, so neither
// belongs inside Card — a caller wraps the rendered Card in a `<Link>`
// or an `ImageLightbox` trigger from outside.
export function Card({ imageSrc, title, subtitle, badge, placeholderIcon: PlaceholderIcon = ImageIcon }: CardProps) {
  return (
    <div className="flex flex-col gap-2">
      <div className="relative aspect-square w-full overflow-hidden rounded-lg border border-border">
        {imageSrc ? (
          <img src={imageSrc} alt={title} className="h-full w-full object-cover" />
        ) : (
          <div
            aria-hidden="true"
            className="flex h-full w-full items-center justify-center bg-surface-raised text-text-secondary"
          >
            <PlaceholderIcon size={32} />
          </div>
        )}
        {badge && <div className="absolute right-2 top-2">{badge}</div>}
      </div>

      <div className="flex flex-col">
        <span className="truncate text-title-md font-medium text-text">{title}</span>
        {subtitle && <span className="truncate text-label text-text-secondary">{subtitle}</span>}
      </div>
    </div>
  )
}
