import { CheckCircle, Circle, Download, Package, SkipForward, AlertCircle } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import type { ItemStatus } from '../../types'

interface StatusConfig {
  icon: LucideIcon
  label: string
  color: string
}

export function statusConfig(status: ItemStatus): StatusConfig {
  switch (status) {
    case 'wanted':
      return { icon: Circle, label: 'Wanted', color: '#60a5fa' }
    case 'grabbed':
      return { icon: Download, label: 'Grabbed', color: '#a78bfa' }
    case 'downloading':
      return { icon: Download, label: 'Downloading', color: '#818cf8' }
    case 'imported':
      return { icon: CheckCircle, label: 'Imported', color: '#34d399' }
    case 'missing':
      return { icon: AlertCircle, label: 'Missing', color: '#f87171' }
    case 'skipped':
      return { icon: SkipForward, label: 'Skipped', color: '#9ca3af' }
    default:
      return { icon: Package, label: status, color: '#9ca3af' }
  }
}

interface Props {
  status: ItemStatus
  monitored: boolean
  size?: 'sm' | 'md'
}

export function ItemStatusBadge({ status, monitored, size = 'sm' }: Props) {
  const cfg = statusConfig(status)
  const Icon = cfg.icon
  const iconSize = size === 'sm' ? 11 : 13
  const opacity = monitored ? 1 : 0.5

  return (
    <span
      className="inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-xs font-medium backdrop-blur-sm"
      style={{
        background: cfg.color + '22',
        color: cfg.color,
        opacity,
      }}
    >
      <Icon size={iconSize} />
      {cfg.label}
    </span>
  )
}
