export interface TextInputProps {
  label: string
  value: string | number
  onChange: (value: string | number) => void
  type?: 'text' | 'number'
  // hint is a short unit/format clarifier appended after the label (e.g.
  // "0.0–1.0" for a threshold, "Go duration, e.g. 45s" for a timeout) —
  // text-text-secondary, same as the label itself, since a unit is real
  // information (docs/design/style-guide.md restricts text-text-muted to
  // disabled/placeholder use only, never anything conveying information).
  hint?: string
  disabled?: boolean
}

// TextInput is the shared string/number field — ADR 0004's "field
// components like Toggle/RuntimeInput" shared-component example. type
// dispatches value parsing: 'number' reports a JS number (via
// valueAsNumber) so a caller never has to re-parse a numeric config key's
// string input value itself.
export function TextInput({ label, value, onChange, type = 'text', hint, disabled }: TextInputProps) {
  return (
    <label className="flex flex-col gap-1 text-body text-text">
      <span className="text-label text-text-secondary">
        {label}
        {hint && ` (${hint})`}
      </span>
      <input
        type={type}
        value={value}
        disabled={disabled}
        onChange={e => onChange(type === 'number' ? e.target.valueAsNumber : e.target.value)}
        className="h-9 px-3 rounded-lg bg-surface-raised border border-border text-text text-body focus:outline-none focus:border-border-hover disabled:opacity-50"
      />
    </label>
  )
}
