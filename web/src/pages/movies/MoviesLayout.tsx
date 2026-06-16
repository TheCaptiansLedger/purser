import { Film, Clapperboard, Zap, Smile, Rocket, Tag } from 'lucide-react'
import { ModuleLayout } from '../../components/layout/ModuleLayout'

const ACCENT = '#3b82f6'

const TABS = [
  { path: '/movies',              label: 'All',    icon: Film,        end: true },
  { path: '/movies/genre/drama',  label: 'Drama',  icon: Clapperboard           },
  { path: '/movies/genre/action', label: 'Action', icon: Zap                    },
  { path: '/movies/genre/comedy', label: 'Comedy', icon: Smile                  },
  { path: '/movies/genre/sci-fi', label: 'Sci-Fi', icon: Rocket                 },
  { path: '/movies/tags',         label: 'Tags',   icon: Tag                    },
]

export function MoviesLayout() {
  return <ModuleLayout tabs={TABS} accent={ACCENT} />
}
