import type { CSSProperties, ReactNode } from 'react'
import type { LucideIcon } from 'lucide-react'

export type ChipTab<T extends string> = {
  id: T
  label: string
  icon?: LucideIcon
}

export function chipTabClassName(active: boolean): string {
  return [
    'flex items-center gap-1.5 px-3 h-8 rounded-lg text-xs font-medium transition-all duration-150',
    active ? 'text-white' : 'text-white/40 hover:text-white/65 hover:bg-white/5',
  ].join(' ')
}

export function chipTabStyle(active: boolean, accent: string): CSSProperties {
  return active ? { background: accent + '28', color: accent } : {}
}

type Props<T extends string> = {
  tabs: ChipTab<T>[]
  value: T
  onChange: (id: T) => void
  accent: string
  rightControls?: ReactNode
}

export function ChipTabs<T extends string>({ tabs, value, onChange, accent, rightControls }: Props<T>) {
  return (
    <div className="flex items-center justify-between mb-8">
      <div className="flex gap-1">
        {tabs.map(({ id, label, icon: Icon }) => (
          <button
            key={id}
            onClick={() => onChange(id)}
            className={chipTabClassName(value === id)}
            style={chipTabStyle(value === id, accent)}
          >
            {Icon && <Icon size={13} />}
            {label}
          </button>
        ))}
      </div>
      {rightControls}
    </div>
  )
}
