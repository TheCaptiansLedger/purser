import { Mic2, Disc3, Tag } from 'lucide-react'
import { ModuleLayout } from '../../components/layout/ModuleLayout'

const ACCENT = '#10b981'

const TABS = [
  { path: '/music',        label: 'Artists', icon: Mic2,  end: true },
  { path: '/music/albums', label: 'Albums',  icon: Disc3            },
  { path: '/music/tags',   label: 'Tags',    icon: Tag              },
]

export function MusicLayout() {
  return <ModuleLayout tabs={TABS} accent={ACCENT} />
}
