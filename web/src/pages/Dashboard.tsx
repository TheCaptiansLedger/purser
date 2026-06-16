import { Film, Tv2, Music2, BookOpen, Sparkles, Users } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { Link } from 'react-router-dom'
import { useLibraryEntries } from '../api/library'
import { usePeople } from '../api/people'
import { useItems } from '../api/items'

const SECTIONS = [
  { path: '/movies',    label: 'Movies',    icon: Film,     accent: '#3b82f6', contentType: 'movie' as const },
  { path: '/tv',        label: 'TV Shows',  icon: Tv2,      accent: '#8b5cf6', contentType: 'tv' as const    },
  { path: '/music',     label: 'Music',     icon: Music2,   accent: '#10b981', contentType: 'music' as const },
  { path: '/books',     label: 'Books',     icon: BookOpen, accent: '#f59e0b', contentType: 'book' as const  },
  { path: '/afterdark', label: 'AfterDark', icon: Sparkles, accent: '#f43f5e', contentType: 'adult' as const },
]

function SummaryCard({ label, count, icon: Icon, accent, path }: { label: string; count?: number; icon: LucideIcon; accent: string; path: string }) {
  return (
    <Link
      to={path}
      className="group relative flex flex-col gap-4 p-6 rounded-2xl border border-white/5 bg-white/3 hover:bg-white/5 hover:border-white/12 transition-all duration-200 overflow-hidden cursor-pointer"
    >
      <div
        className="absolute inset-0 opacity-0 group-hover:opacity-100 transition-opacity duration-300"
        style={{ background: `radial-gradient(ellipse at 0% 0%, ${accent}12 0%, transparent 60%)` }}
      />
      <div
        className="w-10 h-10 rounded-xl flex items-center justify-center shrink-0"
        style={{ background: accent + '22' }}
      >
        <Icon size={20} strokeWidth={2} className="text-white" style={{ color: accent }} />
      </div>
      <div>
        <div className="text-3xl font-semibold text-white tabular-nums">
          {count !== undefined ? count.toLocaleString() : <span className="inline-block w-16 h-8 bg-white/5 rounded animate-pulse" />}
        </div>
        <div className="text-sm text-white/40 mt-0.5">{label}</div>
      </div>
    </Link>
  )
}

export function Dashboard() {
  const movies = useLibraryEntries({ contentType: 'movie', limit: 1 })
  const tv     = useLibraryEntries({ contentType: 'tv',    limit: 1 })
  const music  = useLibraryEntries({ contentType: 'music', limit: 1 })
  const books  = useLibraryEntries({ contentType: 'book',  limit: 1 })
  const adult  = useItems({ contentType: 'adult', limit: 1 })
  const jav    = useItems({ contentType: 'jav',   limit: 1 })
  const people = usePeople({ limit: 1 })

  const adultTotal = (adult.data?.total ?? 0) + (jav.data?.total ?? 0)

  return (
    <div className="px-8 py-10">
      {/* Header */}
      <div className="mb-10">
        <h1 className="text-2xl font-semibold text-white">Library</h1>
        <p className="text-white/40 text-sm mt-1">Your personal media collection</p>
      </div>

      {/* Stats grid */}
      <div className="grid grid-cols-2 lg:grid-cols-3 gap-4 mb-12">
        <SummaryCard label="Movies"    count={movies.data?.total} icon={Film}     accent="#3b82f6" path="/movies"    />
        <SummaryCard label="TV Shows"  count={tv.data?.total}     icon={Tv2}      accent="#8b5cf6" path="/tv"        />
        <SummaryCard label="Albums"    count={music.data?.total}  icon={Music2}   accent="#10b981" path="/music"     />
        <SummaryCard label="Books"     count={books.data?.total}  icon={BookOpen} accent="#f59e0b" path="/books"     />
        <SummaryCard label="AfterDark" count={adultTotal || undefined} icon={Sparkles} accent="#f43f5e" path="/afterdark" />
        <SummaryCard label="People"    count={people.data?.total} icon={Users}    accent="#6366f1" path="/people"    />
      </div>

      {/* Section quick-links */}
      <div>
        <h2 className="text-sm font-medium text-white/40 uppercase tracking-widest mb-4">Sections</h2>
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
          {SECTIONS.map(s => (
            <Link
              key={s.path}
              to={s.path}
              className="flex items-center gap-3 px-4 py-3.5 rounded-xl border border-white/5 bg-white/2 hover:bg-white/5 hover:border-white/10 transition-all duration-150 group"
            >
              <s.icon size={18} strokeWidth={2} style={{ color: s.accent }} />
              <span className="text-sm text-white/70 group-hover:text-white/90 transition-colors">{s.label}</span>
            </Link>
          ))}
        </div>
      </div>
    </div>
  )
}
