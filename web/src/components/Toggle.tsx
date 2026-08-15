export interface ToggleProps {
  label: string
  checked: boolean
  onChange: (checked: boolean) => void
  disabled?: boolean
}

// Toggle is a shared boolean field — ADR 0004 names "field components
// like Toggle/RuntimeInput" explicitly as shared, not page-local. A
// button[role=switch] rather than a hidden checkbox: keyboard/AT users
// get the native switch semantics directly, no sr-only workaround.
//
// Both states carry a visible track border and a thumb color chosen to
// contrast against its own track — a dark thumb on the bright checked
// track, a lighter thumb on the dark unchecked track — rather than
// relying on track fill color alone to read as "on"/"off" at a glance.
// See docs/design/style-guide.md#color for the verified contrast pairs
// this depends on.
export function Toggle({ label, checked, onChange, disabled }: ToggleProps) {
  return (
    <label className="flex items-center justify-between gap-3 text-body text-text">
      <span>{label}</span>
      <button
        type="button"
        role="switch"
        aria-checked={checked}
        aria-label={label}
        disabled={disabled}
        onClick={() => onChange(!checked)}
        className={[
          'relative h-6 w-11 shrink-0 rounded-full border transition-colors',
          checked ? 'bg-accent-system border-accent-system' : 'bg-surface-raised border-border-hover',
          disabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer',
        ].join(' ')}
      >
        <span
          className={[
            'absolute top-0.5 left-0.5 h-5 w-5 rounded-full transition-transform',
            checked ? 'bg-bg translate-x-5' : 'bg-text-secondary translate-x-0',
          ].join(' ')}
        />
      </button>
    </label>
  )
}
