import { Globe, Building2, Film, Users, Tag } from 'lucide-react'
import { ModuleLayout } from '../../components/layout/ModuleLayout'

const ACCENT = '#f43f5e'

const TABS = [
  { path: '/afterdark/networks',   label: 'Networks',   icon: Globe      },
  { path: '/afterdark/studios',    label: 'Studios',    icon: Building2  },
  { path: '/afterdark/scenes',     label: 'Scenes',     icon: Film       },
  { path: '/afterdark/performers', label: 'Performers', icon: Users      },
  { path: '/afterdark/tags',       label: 'Tags',       icon: Tag        },
]

export function AfterDarkLayout() {
  return <ModuleLayout tabs={TABS} accent={ACCENT} />
}
