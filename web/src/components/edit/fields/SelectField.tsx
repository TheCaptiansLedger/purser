export interface SelectOption {
  value: string
  label: string
}

interface SelectFieldProps {
  value: string
  onChange: (value: string) => void
  options: SelectOption[]
  disabled?: boolean
}

export function SelectField({ value, onChange, options, disabled }: SelectFieldProps) {
  return (
    <select
      value={value}
      onChange={e => onChange(e.target.value)}
      disabled={disabled}
      className="w-full rounded-lg border border-white/10 bg-zinc-800 px-3 py-2 text-sm text-white transition-colors focus:border-white/25 focus:outline-none disabled:opacity-50"
    >
      {options.map(opt => (
        <option key={opt.value} value={opt.value}>{opt.label}</option>
      ))}
    </select>
  )
}
