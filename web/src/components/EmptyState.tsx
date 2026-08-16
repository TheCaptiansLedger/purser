import type { LucideIcon } from 'lucide-react'

export interface EmptyStateAction {
  label: string
  onClick?: () => void
  disabled?: boolean
}

export interface EmptyStateProps {
  icon: LucideIcon
  title: string
  description: string
  action?: EmptyStateAction
}

// EmptyState — style-guide Component vocabulary entry
// (docs/design/style-guide.md#component-vocabulary): zero-results and
// empty-library states, always naming what's missing and offering a next
// action (NN/g heuristic 9) rather than a bare "Nothing here." Content-
// type agnostic — every call site supplies its own icon/copy/action, no
// module-specific branch lives in here (ADR 0002 SRP/OCP, same shape as
// PersonCard).
//
// action is optional: a zero-results state (search/filter yielded
// nothing on a non-empty library) has no next action beyond "clear the
// filter," which the caller already renders elsewhere.
export function EmptyState({ icon: Icon, title, description, action }: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center gap-3 px-6 py-16 text-center">
      <Icon size={40} className="text-text-secondary" aria-hidden="true" />
      <p className="text-title-md font-semibold text-text">{title}</p>
      <p className="max-w-sm text-body text-text-secondary">{description}</p>
      {action && (
        <button
          type="button"
          onClick={action.onClick}
          disabled={action.disabled}
          title={action.disabled ? 'Coming soon' : undefined}
          className="mt-2 h-9 px-4 rounded-lg bg-accent-system text-bg text-body font-medium hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {action.label}
        </button>
      )}
    </div>
  )
}
