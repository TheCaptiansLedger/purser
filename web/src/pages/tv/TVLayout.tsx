import { Tv2, Clapperboard, Globe, Tag } from 'lucide-react'
import { ModuleLayout } from '../../components/layout/ModuleLayout'

const ACCENT = '#8b5cf6'

const TABS = [
  { path: '/tv',           label: 'All Shows', icon: Tv2,         end: true },
  { path: '/tv/genres',    label: 'Genres',    icon: Clapperboard           },
  { path: '/tv/networks',  label: 'Networks',  icon: Globe                  },
  { path: '/tv/tags',      label: 'Tags',      icon: Tag                    },
]

export function TVLayout() {
  return <ModuleLayout tabs={TABS} accent={ACCENT} />
}
