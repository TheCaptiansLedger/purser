import { Lock } from 'lucide-react'

export interface LockBadgeProps {
  reason: 'operator' | 'bootstrap'
}

// LockBadge explains why a locked Setting can't be edited from the DB
// overlay — the two messages docs/adr/0028-layered-settings.md specifies:
// "set via env/yaml" for an operator-locked key, "requires a restart" for
// a bootstrap-locked one (today, database.* only — see
// web/src/pages/settings/settingsCategory.ts).
export function LockBadge({ reason }: LockBadgeProps) {
  const label = reason === 'bootstrap' ? 'Requires restart' : 'Set via env/yaml'
  return (
    <span className="inline-flex items-center gap-1 h-5 px-1.5 rounded-sm bg-surface-raised text-label text-text-secondary">
      <Lock size={10} />
      {label}
    </span>
  )
}
