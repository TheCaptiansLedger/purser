import type { LucideIcon } from 'lucide-react'

interface Props {
  icon: LucideIcon
  title: string
  description?: string
  accent?: string
}

export function EmptyState({ icon: Icon, title, description, accent }: Props) {
  return (
    <div className="flex flex-col items-center justify-center py-24 px-8 text-center">
      <div
        className="w-16 h-16 rounded-2xl flex items-center justify-center mb-4"
        style={{ background: accent ? accent + '18' : 'rgba(255,255,255,0.05)' }}
      >
        <Icon size={28} style={{ color: accent ?? 'rgba(255,255,255,0.3)' }} strokeWidth={1.5} />
      </div>
      <h3 className="text-white/70 font-medium text-base mb-1">{title}</h3>
      {description && <p className="text-white/35 text-sm max-w-xs">{description}</p>}
    </div>
  )
}
