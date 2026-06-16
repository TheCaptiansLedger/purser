import { Search, X } from 'lucide-react'

interface Props {
  value: string
  onChange: (v: string) => void
  placeholder?: string
  accent?: string
}

export function SearchBar({ value, onChange, placeholder = 'Search…', accent }: Props) {
  return (
    <div className="relative group">
      <Search size={15} className="absolute left-3 top-1/2 -translate-y-1/2 text-white/30 pointer-events-none" />
      <input
        type="text"
        value={value}
        onChange={e => onChange(e.target.value)}
        placeholder={placeholder}
        className={[
          'w-full h-9 pl-9 pr-8 rounded-lg text-sm',
          'bg-white/5 border border-white/8',
          'text-white placeholder:text-white/25',
          'outline-none transition-all duration-150',
          'focus:bg-white/8 focus:border-white/20',
        ].join(' ')}
        style={accent ? { '--tw-ring-color': accent } as React.CSSProperties : {}}
      />
      {value && (
        <button
          onClick={() => onChange('')}
          className="absolute right-2.5 top-1/2 -translate-y-1/2 text-white/30 hover:text-white/60 transition-colors"
        >
          <X size={14} />
        </button>
      )}
    </div>
  )
}
