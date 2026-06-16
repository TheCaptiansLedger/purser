import { useState } from 'react'
import { Users, Building2, Film } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { usePeople } from '../../api/people'
import { useLibraryEntries } from '../../api/library'
import { useItems } from '../../api/items'
import { PageHeader } from '../../components/layout/PageHeader'
import { PersonCard } from '../../components/media/PersonCard'
import { EntryCard } from '../../components/media/EntryCard'
import { ItemCard } from '../../components/media/ItemCard'
import { Pagination } from '../../components/ui/Pagination'
import { SkeletonGrid } from '../../components/ui/Skeleton'
import { EmptyState } from '../../components/ui/EmptyState'

const ACCENT = '#f43f5e'
const LIMIT = 48

type Tab = 'performers' | 'studios' | 'scenes'

const TABS: { id: Tab; label: string; icon: LucideIcon }[] = [
  { id: 'performers', label: 'Performers', icon: Users },
  { id: 'studios',    label: 'Studios',    icon: Building2 },
  { id: 'scenes',     label: 'Scenes',     icon: Film },
]

export function AfterDarkPage() {
  const [tab, setTab] = useState<Tab>('performers')
  const [search, setSearch] = useState('')
  const [offset, setOffset] = useState(0)

  const resetPage = (v: string) => { setSearch(v); setOffset(0) }
  const switchTab = (t: Tab) => { setTab(t); setSearch(''); setOffset(0) }

  const performers = usePeople({
    contentType: 'adult',
    search: search || undefined,
    limit: LIMIT,
    offset,
  })

  const studios = useLibraryEntries({
    contentType: 'adult',
    search: search || undefined,
    limit: LIMIT,
    offset,
  })

  const adultScenes = useItems({
    contentType: 'adult',
    search: search || undefined,
    limit: LIMIT,
    offset,
  })

  const javScenes = useItems({
    contentType: 'jav',
    search: search || undefined,
    limit: tab === 'scenes' ? LIMIT : 1,
    offset,
  })

  // Merge adult+jav for scenes (show adult first up to LIMIT)
  const scenesData = adultScenes.data
  const scenesLoading = adultScenes.isLoading || javScenes.isLoading

  const total = tab === 'performers' ? performers.data?.total
    : tab === 'studios' ? studios.data?.total
    : scenesData?.total

  return (
    <div>
      <PageHeader
        title="AfterDark"
        accent={ACCENT}
        search={search}
        onSearch={resetPage}
        total={total}
      >
        {/* Sub-nav tabs */}
        <div className="flex gap-1 shrink-0">
          {TABS.map(({ id, label, icon: Icon }) => (
            <button
              key={id}
              onClick={() => switchTab(id)}
              className={[
                'flex items-center gap-1.5 px-3 h-8 rounded-lg text-xs font-medium transition-all duration-150',
                tab === id
                  ? 'text-white'
                  : 'text-white/40 hover:text-white/65 hover:bg-white/5',
              ].join(' ')}
              style={tab === id ? { background: ACCENT + '28', color: ACCENT } : {}}
            >
              <Icon size={13} />
              {label}
            </button>
          ))}
        </div>
      </PageHeader>

      <div className="px-8 py-6">
        {/* Performers */}
        {tab === 'performers' && (
          performers.isLoading ? <SkeletonGrid count={24} aspect="2/3" /> :
          !performers.data?.data.length ? (
            <EmptyState icon={Users} title="No performers yet" accent={ACCENT} />
          ) : (
            <>
              <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4">
                {performers.data.data.map(p => (
                  <PersonCard key={p.id} person={p} href={`/afterdark/performers/${p.id}`} accent={ACCENT} />
                ))}
              </div>
              <Pagination total={performers.data.total} limit={LIMIT} offset={offset} onChange={setOffset} accent={ACCENT} />
            </>
          )
        )}

        {/* Studios */}
        {tab === 'studios' && (
          studios.isLoading ? <SkeletonGrid count={24} aspect="16/9" /> :
          !studios.data?.data.length ? (
            <EmptyState icon={Building2} title="No studios yet" accent={ACCENT} />
          ) : (
            <>
              <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-4">
                {studios.data.data.map(e => (
                  <EntryCard key={e.id} entry={e} href={`/afterdark/studios/${e.id}`} aspect="16/9" accent={ACCENT} />
                ))}
              </div>
              <Pagination total={studios.data.total} limit={LIMIT} offset={offset} onChange={setOffset} accent={ACCENT} />
            </>
          )
        )}

        {/* Scenes */}
        {tab === 'scenes' && (
          scenesLoading ? <SkeletonGrid count={24} aspect="16/9" /> :
          !scenesData?.data.length ? (
            <EmptyState icon={Film} title="No scenes yet" accent={ACCENT} />
          ) : (
            <>
              <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-4">
                {scenesData.data.map(item => (
                  <ItemCard key={item.id} item={item} href={`/afterdark/scenes/${item.id}`} aspect="16/9" accent={ACCENT} showPeople />
                ))}
              </div>
              <Pagination total={scenesData.total} limit={LIMIT} offset={offset} onChange={setOffset} accent={ACCENT} />
            </>
          )
        )}
      </div>
    </div>
  )
}
