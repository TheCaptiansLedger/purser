import { Settings, Database } from 'lucide-react'
import { ModuleLayout } from '../../components/layout/ModuleLayout'

const ACCENT = '#6366f1'

const TABS = [
  { path: '/settings/config',   label: 'Config',   icon: Settings },
  { path: '/settings/database', label: 'Database', icon: Database },
]

export function SettingsLayout() {
  return <ModuleLayout tabs={TABS} accent={ACCENT} />
}
