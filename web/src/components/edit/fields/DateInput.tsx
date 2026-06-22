interface DateInputProps {
  value: string
  onChange: (value: string) => void
  disabled?: boolean
}

export function DateInput({ value, onChange, disabled }: DateInputProps) {
  return (
    <input
      type="date"
      value={value}
      onChange={e => onChange(e.target.value)}
      disabled={disabled}
      className="w-full rounded-lg border border-white/10 bg-white/5 px-3 py-2 text-sm text-white transition-colors focus:border-white/25 focus:outline-none disabled:opacity-50"
    />
  )
}
