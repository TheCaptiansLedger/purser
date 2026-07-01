import { useState } from 'react'
import { Users, Plus } from 'lucide-react'
import { useQueryClient } from '@tanstack/react-query'
import { usePeople, usePeopleRoles } from '../../api/people'
import { createPerson } from '../../api/people'
import type { CreatePersonRequest } from '../../api/people'
import { PageHeader } from '../../components/layout/PageHeader'
import { PersonCard } from '../../components/media/PersonCard'
import { Pagination } from '../../components/ui/Pagination'
import { SkeletonGrid } from '../../components/ui/Skeleton'
import { EmptyState } from '../../components/ui/EmptyState'
import { ChipTabs } from '../../components/ui/ChipTabs'
import type { ChipTab } from '../../components/ui/ChipTabs'
import { AddPersonDialog } from './AddPersonDialog'
import type { PersonRole } from '../../types'

const ACCENT = '#6366f1'
const LIMIT = 48

type RoleTab = PersonRole | 'all'

const ROLE_LABELS: Record<PersonRole, string> = {
  performer: 'Performers',
  actress: 'Actresses',
  director: 'Directors',
  actor: 'Actors',
  artist: 'Artists',
  producer: 'Producers',
  author: 'Authors',
}

export function PeoplePage() {
  const queryClient = useQueryClient()
  const [search, setSearch] = useState('')
  const [offset, setOffset] = useState(0)
  const [showAdd, setShowAdd] = useState(false)
  const [activeTab, setActiveTab] = useState<RoleTab>('all')

  const { data: rolesData } = usePeopleRoles()

  const roleFilter = activeTab === 'all' ? undefined : activeTab

  const { data, isLoading } = usePeople({
    search: search || undefined,
    role: roleFilter,
    limit: LIMIT,
    offset,
  })

  function handleTabChange(tab: RoleTab) {
    setActiveTab(tab)
    setOffset(0)
  }

  async function handleAdd(req: CreatePersonRequest) {
    await createPerson(req)
    queryClient.invalidateQueries({ queryKey: ['people'] })
  }

  const tabs: ChipTab<RoleTab>[] = [
    { id: 'all', label: 'All' },
    ...(rolesData ?? []).map(rc => ({
      id: rc.role as RoleTab,
      label: ROLE_LABELS[rc.role] ?? rc.role,
    })),
  ]

  return (
    <div>
      <PageHeader title="People" accent={ACCENT} search={search} onSearch={v => { setSearch(v); setOffset(0) }} total={data?.total}>
        <button
          onClick={() => setShowAdd(true)}
          className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium text-white transition-colors shrink-0"
          style={{ background: ACCENT }}
        >
          <Plus size={14} />
          Add Person
        </button>
      </PageHeader>
      <div className="px-8 py-6">
        {tabs.length > 1 && (
          <ChipTabs tabs={tabs} value={activeTab} onChange={handleTabChange} accent={ACCENT} />
        )}
        {isLoading ? (
          <SkeletonGrid count={24} aspect="2/3" />
        ) : !data?.data.length ? (
          <EmptyState icon={Users} title="No people yet" description="People appear here when they are linked to items in your library." accent={ACCENT} />
        ) : (
          <>
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4">
              {data.data.map(p => (
                <PersonCard key={p.id} person={p} href={`/people/${p.id}`} accent={ACCENT} />
              ))}
            </div>
            <Pagination total={data.total} limit={LIMIT} offset={offset} onChange={setOffset} accent={ACCENT} />
          </>
        )}
      </div>

      <AddPersonDialog
        open={showAdd}
        onClose={() => setShowAdd(false)}
        accent={ACCENT}
        onAdd={handleAdd}
      />
    </div>
  )
}
