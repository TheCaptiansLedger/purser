import { Settings, Database, ListTodo, Gauge, Inbox } from 'lucide-react'
import { ModuleLayout } from '../../components/layout/ModuleLayout'
import { useUnmatchedCount } from '../../api/scan'

const ACCENT = '#6366f1'

export function SettingsLayout() {
  const pendingCount = useUnmatchedCount().data ?? 0

  const TABS = [
    { path: '/settings/config',       label: 'Config',       icon: Settings },
    { path: '/settings/database',     label: 'Database',     icon: Database },
    { path: '/settings/jobs',         label: 'Jobs',         icon: ListTodo },
    { path: '/settings/cache',        label: 'Cache',        icon: Gauge },
    { path: '/settings/import-queue', label: 'Import Queue', icon: Inbox, badge: pendingCount },
  ]

  return <ModuleLayout tabs={TABS} accent={ACCENT} />
}
