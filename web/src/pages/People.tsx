import { useMutation } from '@connectrpc/connect-query'
import { CheckSquare, Plus, Search, SearchX, UserPlus } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { BulkDeleteDialog } from '../components/BulkDeleteDialog'
import { DropdownMenu } from '../components/DropdownMenu'
import { EmptyState } from '../components/EmptyState'
import { PersonCard } from '../components/PersonCard'
import { PersonDialog } from '../components/PersonDialog'
import { PersonSearchDialog } from '../components/PersonSearchDialog'
import { SelectableTile } from '../components/SelectableTile'
import { SelectionToolbar } from '../components/SelectionToolbar'
import { Toggle } from '../components/Toggle'
import { bulkDeletePeople } from '../gen/purser/domain/v1/person-PersonService_connectquery'
import { usePeopleList } from '../hooks/usePeopleList'
import { usePersonDeletionImpacts } from '../hooks/usePersonDeletionImpacts'
import { usePersonImages } from '../hooks/usePersonImages'

// SEARCH_DEBOUNCE_MS keeps ListPeople(name=query) (#653) from firing on
// every keystroke — the search box calls the server, not a client-side
// filter over whatever page happens to be loaded, per this issue's (#660)
// scope note.
const SEARCH_DEBOUNCE_MS = 300

// People — the People index page (#660). PersonService.ListPeople,
// paginated (usePeopleList), rendered as a PersonCard (#657) grid. Search
// is server-side (#653's name filter); "Monitored only" has no
// server-side equivalent (issue #660's own scope note: filters the
// loaded page client-side only, a real limitation accepted at this
// scale — a future issue adds a server-side filter the same way #653
// added name, if it becomes a problem).
//
// Bulk delete: "Select" swaps every card's click-target-detection div for
// SelectableTile's toggle-button one (via onNavigate/ariaLabel — a plain
// <Link> wrap isn't valid here since PersonCard's photo is itself a
// nested button, same reason the non-select-mode path below never used
// <Link> either), exposing a SelectionToolbar; its "Delete" opens
// BulkDeleteDialog against usePersonDeletionImpacts(selectedIds) and
// BulkDeletePeople — see docs/adr/0015/0016. Person never blocks a delete
// (internal/service/person_deletion.go), so the dialog's cascade checkbox
// never actually appears here, same as the Discography tab's Group case.
//
// "Add Person" (#663, #723) is a DropdownMenu with two sources: "Search
// MusicBrainz" opens PersonSearchDialog (get-or-create by mbid, ADR
// 0026), "Add Manually" opens the existing PersonDialog in create mode —
// same two-entry-point shape Music Library's own "Add Artist" menu
// already established (#722). Editing an existing Person stays manual
// only, unaffected by this.
export function People() {
  const navigate = useNavigate()
  const [searchInput, setSearchInput] = useState('')
  const [name, setName] = useState('')
  const [monitoredOnly, setMonitoredOnly] = useState(false)
  const [addDialogOpen, setAddDialogOpen] = useState(false)
  const [searchDialogOpen, setSearchDialogOpen] = useState(false)
  const [selectMode, setSelectMode] = useState(false)
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [bulkDeleteOpen, setBulkDeleteOpen] = useState(false)

  useEffect(() => {
    const timer = setTimeout(() => setName(searchInput.trim()), SEARCH_DEBOUNCE_MS)
    return () => clearTimeout(timer)
  }, [searchInput])

  const { data, isPending, isError, error, hasNextPage, isFetchingNextPage, fetchNextPage, refetch } =
    usePeopleList(name)

  const people = useMemo(() => data?.pages.flatMap(page => page.people) ?? [], [data])
  const visiblePeople = useMemo(
    () => (monitoredOnly ? people.filter(person => person.monitored) : people),
    [people, monitoredOnly],
  )

  const personIds = useMemo(() => visiblePeople.map(person => person.id), [visiblePeople])
  const imagesByPersonId = usePersonImages(personIds)

  // Empty-library (no query, no results at all) vs zero-results (a
  // search/filter that matched nothing) get distinct EmptyState copy —
  // the former offers "Add Person," the latter doesn't (clearing the
  // search/toggle is the only relevant next action, and that control is
  // already on screen).
  const isEmptyLibrary = !isPending && !isError && people.length === 0 && name === '' && !monitoredOnly
  const isZeroResults = !isPending && !isError && people.length > 0 && visiblePeople.length === 0
  const isZeroSearchResults = !isPending && !isError && people.length === 0 && (name !== '' || monitoredOnly)

  const selectedIdList = useMemo(() => Array.from(selectedIds), [selectedIds])
  const impact = usePersonDeletionImpacts(bulkDeleteOpen ? selectedIdList : [])
  const bulkDeleteMutation = useMutation(bulkDeletePeople)

  function toggleSelected(id: string) {
    setSelectedIds(prev => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      return next
    })
  }

  function exitSelectMode() {
    setSelectMode(false)
    setSelectedIds(new Set())
  }

  function handleBulkDelete(cascade: boolean) {
    bulkDeleteMutation.mutate(
      { ids: selectedIdList, cascade },
      {
        onSuccess: () => {
          setBulkDeleteOpen(false)
          exitSelectMode()
          void refetch()
        },
      },
    )
  }

  return (
    <div className="px-6 py-10 md:px-8">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <h1 className="text-headline text-text">People</h1>

        <div className="flex flex-wrap items-center gap-3">
          <label className="relative">
            <Search
              size={16}
              className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-text-secondary"
              aria-hidden="true"
            />
            <input
              type="text"
              value={searchInput}
              onChange={e => setSearchInput(e.target.value)}
              placeholder="Search people…"
              aria-label="Search people"
              className="h-9 w-40 rounded-lg border border-border bg-surface-raised pl-8 pr-3 text-body text-text focus:outline-none focus:border-border-hover sm:w-52 md:w-64"
            />
          </label>

          <Toggle label="Monitored only" checked={monitoredOnly} onChange={setMonitoredOnly} />

          <button
            type="button"
            onClick={() => setSelectMode(true)}
            className="flex h-9 items-center gap-1.5 rounded-lg border border-border px-4 text-body font-medium text-text hover:bg-surface-raised"
          >
            <CheckSquare size={16} aria-hidden="true" />
            Select
          </button>

          <DropdownMenu
            label="Add Person"
            trigger={
              <>
                <Plus size={16} aria-hidden="true" />
                Add Person
              </>
            }
            triggerClassName="flex h-9 items-center gap-1.5 rounded-lg bg-accent-system px-4 text-body font-medium text-bg hover:opacity-90"
            items={[
              { label: 'Search MusicBrainz', onSelect: () => setSearchDialogOpen(true) },
              { label: 'Add Manually', onSelect: () => setAddDialogOpen(true) },
            ]}
          />
        </div>
      </div>

      {selectMode && (
        <SelectionToolbar
          count={selectedIds.size}
          entityLabelPlural="people"
          onDelete={() => setBulkDeleteOpen(true)}
          onCancel={exitSelectMode}
        />
      )}

      {bulkDeleteOpen && (
        <BulkDeleteDialog
          entityLabelPlural="people"
          count={selectedIds.size}
          impact={impact}
          onDelete={handleBulkDelete}
          isDeleting={bulkDeleteMutation.isPending}
          deleteError={bulkDeleteMutation.isError ? `Couldn't delete these people (${bulkDeleteMutation.error.message}).` : undefined}
          onClose={() => setBulkDeleteOpen(false)}
        />
      )}

      {isError && (
        <p className="mt-6 text-body text-status-failure">
          Couldn't load people ({error.message}).
        </p>
      )}

      {isEmptyLibrary && (
        <EmptyState
          icon={UserPlus}
          title="No people yet"
          description="Add a person to start building your People library."
        />
      )}

      {(isZeroResults || isZeroSearchResults) && (
        <EmptyState
          icon={SearchX}
          title="No matches"
          description="No people match your search or filter."
        />
      )}

      {visiblePeople.length > 0 && (
        <div className="mt-6 grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 xl:grid-cols-8 2xl:grid-cols-10">
          {visiblePeople.map(person => (
            <SelectableTile
              key={person.id}
              selectMode={selectMode}
              selected={selectedIds.has(person.id)}
              onToggle={() => toggleSelected(person.id)}
              onNavigate={() => navigate(`/people/${person.id}`)}
              ariaLabel={`View ${person.name}`}
            >
              <PersonCard person={{ id: person.id, name: person.name, imageId: imagesByPersonId[person.id] }} />
            </SelectableTile>
          ))}
        </div>
      )}

      {hasNextPage && (
        <div className="mt-6 flex justify-center">
          <button
            type="button"
            onClick={() => fetchNextPage()}
            disabled={isFetchingNextPage}
            className="h-9 px-4 rounded-lg bg-surface-raised border border-border text-body font-medium text-text hover:bg-surface disabled:opacity-50"
          >
            {isFetchingNextPage ? 'Loading…' : 'Load more'}
          </button>
        </div>
      )}

      {addDialogOpen && (
        <PersonDialog
          mode="create"
          onClose={() => setAddDialogOpen(false)}
          onSaved={person => {
            setAddDialogOpen(false)
            navigate(`/people/${person.id}`)
          }}
        />
      )}

      {searchDialogOpen && (
        <PersonSearchDialog
          onClose={() => setSearchDialogOpen(false)}
          onAdded={personId => {
            setSearchDialogOpen(false)
            navigate(`/people/${personId}`)
          }}
        />
      )}
    </div>
  )
}
