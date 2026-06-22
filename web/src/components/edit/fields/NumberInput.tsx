interface NumberInputProps {
  value: number | ''
  onChange: (value: number | '') => void
  placeholder?: string
  min?: number
  max?: number
  disabled?: boolean
}

export function NumberInput({ value, onChange, placeholder, min, max, disabled }: NumberInputProps) {
  return (
    <input
      type="number"
      value={value}
      onChange={e => onChange(e.target.value === '' ? '' : Number(e.target.value))}
      placeholder={placeholder}
      min={min}
      max={max}
      disabled={disabled}
      className="w-full rounded-lg border border-white/10 bg-white/5 px-3 py-2 text-sm text-white placeholder:text-white/25 transition-colors focus:border-white/25 focus:outline-none disabled:opacity-50"
    />
  )
}
