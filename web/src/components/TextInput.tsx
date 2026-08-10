export interface TextInputProps {
  label: string
  value: string | number
  onChange: (value: string | number) => void
  type?: 'text' | 'number'
  disabled?: boolean
}

// TextInput is the shared string/number field — ADR 0004's "field
// components like Toggle/RuntimeInput" shared-component example. type
// dispatches value parsing: 'number' reports a JS number (via
// valueAsNumber) so a caller never has to re-parse a numeric config key's
// string input value itself.
export function TextInput({ label, value, onChange, type = 'text', disabled }: TextInputProps) {
  return (
    <label className="flex flex-col gap-1 text-body text-text">
      <span className="text-label text-text-secondary">{label}</span>
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
