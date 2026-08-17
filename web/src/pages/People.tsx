import { Search, SearchX, UserPlus } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { EmptyState } from '../components/EmptyState'
import { PersonCard } from '../components/PersonCard'
import { Toggle } from '../components/Toggle'
import { usePeopleList } from '../hooks/usePeopleList'
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
export function People() {
  const navigate = useNavigate()
  const [searchInput, setSearchInput] = useState('')
  const [name, setName] = useState('')
  const [monitoredOnly, setMonitoredOnly] = useState(false)

  useEffect(() => {
    const timer = setTimeout(() => setName(searchInput.trim()), SEARCH_DEBOUNCE_MS)
    return () => clearTimeout(timer)
  }, [searchInput])

  const { data, isPending, isError, error, hasNextPage, isFetchingNextPage, fetchNextPage } =
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
        </div>
      </div>

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
          action={{ label: 'Add Person', disabled: true }}
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
            // A plain clickable div, not <Link>, because PersonCard's photo
            // is itself a <button> (opens the lightbox) — nesting a button
            // inside an anchor is invalid HTML. role="link"/tabIndex/Enter
            // keeps it keyboard-reachable; a click that lands on the photo
            // button is left alone so the lightbox still opens instead of
            // navigating (checked via closest('button')).
            <div
              key={person.id}
              role="link"
              tabIndex={0}
              aria-label={`View ${person.name}`}
              onClick={e => {
                if ((e.target as HTMLElement).closest('button')) return
                navigate(`/people/${person.id}`)
              }}
              onKeyDown={e => {
                if (e.key === 'Enter') navigate(`/people/${person.id}`)
              }}
              className="cursor-pointer rounded-lg hover:bg-surface-raised"
            >
              <PersonCard person={{ id: person.id, name: person.name, imageId: imagesByPersonId[person.id] }} />
            </div>
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
    </div>
  )
}
