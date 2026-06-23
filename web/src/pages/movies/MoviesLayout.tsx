import { Film, Clapperboard, Tag } from 'lucide-react'
import { ModuleLayout } from '../../components/layout/ModuleLayout'

const ACCENT = '#3b82f6'

const TABS = [
  { path: '/movies',         label: 'All',    icon: Film,        end: true },
  { path: '/movies/genres',  label: 'Genres', icon: Clapperboard           },
  { path: '/movies/tags',    label: 'Tags',   icon: Tag                    },
]

export function MoviesLayout() {
  return <ModuleLayout tabs={TABS} accent={ACCENT} />
}
