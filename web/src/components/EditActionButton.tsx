import { ChevronDown, Pencil } from 'lucide-react'
import { DropdownMenu, type DropdownMenuItem } from './DropdownMenu'

export interface EditActionButtonProps {
  // onEdit opens the entity's manual field editor (#681 — not yet built
  // for any entity). Omit it to render the primary segment disabled
  // rather than wiring it to a silent no-op; every caller does this until
  // its own entity's editor exists.
  onEdit?: () => void
  editDisabledReason?: string
  // items — one entry per external-provider action (#671's "Refresh from
  // MusicBrainz" is the first), exposed behind the chevron segment rather
  // than as separate buttons. Same one-menu-item-per-provider vocabulary
  // DropdownMenu's own first consumer (Add Album's source picker) already
  // established — no ranking/merging across providers, per ADR 0027.
  items: DropdownMenuItem[]
}

// EditActionButton is this module's one "Edit ▾" split-button convention
// (the shared-component vocabulary ADR 0004 already names `EditButton` —
// built here as the first concrete instance, on Artist Detail via #671,
// general enough for #681's Person/Album editors to reuse without a
// second control being invented). Left segment: primary "Edit" action.
// Right segment: a DropdownMenu of provider-refresh actions.
//
// The pill look comes from rounding each segment's own outer corner
// (`rounded-l-lg`/`rounded-r-lg`), not `overflow-hidden` on the wrapper —
// `overflow-hidden` here would clip DropdownMenu's absolutely-positioned
// popover to this row's own height, since overflow clipping applies to
// positioned descendants too, not just normal-flow content. That clipping
// is what made the menu unable to show its full item text.
export function EditActionButton({ onEdit, editDisabledReason, items }: EditActionButtonProps) {
  return (
    <div className="flex h-8 items-stretch rounded-lg border border-border">
      <button
        type="button"
        onClick={onEdit}
        disabled={!onEdit}
        title={!onEdit ? editDisabledReason : undefined}
        className="flex items-center gap-1.5 rounded-l-lg px-3 text-label font-medium text-text-secondary hover:bg-surface-raised hover:text-text disabled:opacity-50 disabled:hover:bg-transparent"
      >
        <Pencil size={14} />
        Edit
      </button>
      <DropdownMenu
        label="More edit actions"
        items={items}
        trigger={<ChevronDown size={14} />}
        triggerClassName="flex h-full items-center justify-center rounded-r-lg border-l border-border px-2 text-text-secondary hover:bg-surface-raised hover:text-text"
      />
    </div>
  )
}
