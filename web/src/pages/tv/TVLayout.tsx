import { Tv2, Clapperboard, Smile, Shield, Globe, Tag } from 'lucide-react'
import { ModuleLayout } from '../../components/layout/ModuleLayout'

const ACCENT = '#8b5cf6'

const TABS = [
  { path: '/tv',               label: 'All Shows', icon: Tv2,         end: true },
  { path: '/tv/genre/drama',   label: 'Drama',     icon: Clapperboard           },
  { path: '/tv/genre/comedy',  label: 'Comedy',    icon: Smile                  },
  { path: '/tv/genre/crime',   label: 'Crime',     icon: Shield                 },
  { path: '/tv/networks',      label: 'Networks',  icon: Globe                  },
  { path: '/tv/tags',          label: 'Tags',      icon: Tag                    },
]

export function TVLayout() {
  return <ModuleLayout tabs={TABS} accent={ACCENT} />
}
