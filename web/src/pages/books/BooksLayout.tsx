import { BookOpen, BookMarked, User, Tag } from 'lucide-react'
import { ModuleLayout } from '../../components/layout/ModuleLayout'

const ACCENT = '#f59e0b'

const TABS = [
  { path: '/books',         label: 'Books',   icon: BookOpen,   end: true },
  { path: '/books/series',  label: 'Series',  icon: BookMarked            },
  { path: '/books/authors', label: 'Authors', icon: User                  },
  { path: '/books/tags',    label: 'Tags',    icon: Tag                   },
]

export function BooksLayout() {
  return <ModuleLayout tabs={TABS} accent={ACCENT} />
}
