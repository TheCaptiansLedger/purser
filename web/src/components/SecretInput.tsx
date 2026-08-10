export interface SecretInputProps {
  label: string
  isSet: boolean
  value: string
  onChange: (value: string) => void
  disabled?: boolean
}

// SecretInput is the shared write-only secret field (API keys, passwords,
// DSNs) — see docs/adr/0028-layered-settings.md's "Secret masking"
// section. value is always the caller's in-progress draft, never the real
// stored secret (GetSettings never returns it); isSet only changes the
// placeholder copy, distinguishing "configured, replace to change" from
// "never configured."
export function SecretInput({ label, isSet, value, onChange, disabled }: SecretInputProps) {
  return (
    <label className="flex flex-col gap-1 text-body text-text">
      <span className="text-label text-text-secondary">{label}</span>
      <input
        type="password"
        value={value}
        placeholder={isSet ? 'Set — enter a new value to replace' : 'Not set'}
        disabled={disabled}
        onChange={e => onChange(e.target.value)}
        autoComplete="new-password"
        className="h-9 px-3 rounded-lg bg-surface-raised border border-border text-text text-body placeholder:text-text-muted focus:outline-none focus:border-border-hover disabled:opacity-50"
      />
    </label>
  )
}
