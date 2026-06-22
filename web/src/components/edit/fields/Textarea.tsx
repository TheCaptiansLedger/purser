interface TextareaProps {
  value: string
  onChange: (value: string) => void
  placeholder?: string
  rows?: number
  disabled?: boolean
}

export function Textarea({ value, onChange, placeholder, rows = 4, disabled }: TextareaProps) {
  return (
    <textarea
      value={value}
      onChange={e => onChange(e.target.value)}
      placeholder={placeholder}
      rows={rows}
      disabled={disabled}
      className="w-full resize-none rounded-lg border border-white/10 bg-white/5 px-3 py-2 text-sm text-white placeholder:text-white/25 transition-colors focus:border-white/25 focus:outline-none disabled:opacity-50"
    />
  )
}
