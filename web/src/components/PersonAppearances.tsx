import { useMemo } from 'react'
import { useEntryPeopleList } from '../hooks/useEntryPeopleList'
import { useItemPeopleList } from '../hooks/useItemPeopleList'
import { useLibraryEntriesByIds } from '../hooks/useLibraryEntriesByIds'
import { useItemsByIds } from '../hooks/useItemsByIds'

export interface PersonAppearancesProps {
  personId: string
}

interface AppearanceRow {
  key: string
  role: string
  name: string
}

// PersonAppearances — the Person Detail page's (#661) "Appears as"
// cross-module list (#662). EntryPersonService.ListEntryPeople(person_id)
// and ItemPersonService.ListItemPeople(person_id), each row resolved to
// its LibraryEntry/Item via Get, rendered generically as role + linked-
// entity name. No content-type switch statement (ADR 0001 self-audit
// item 1): a row's `name` comes from whichever generic field
// (LibraryEntry.name or Item.title) its own Get response has — this
// component never reads contentType/kind to decide how to render it.
//
// No <Link> yet: neither a /library-entries/:id nor an /items/:id
// detail route exists (see docs/design/frontend-stack.md — no module
// screens are built yet), so each row is plain text for now rather than
// a link to a route that 404s.
export function PersonAppearances({ personId }: PersonAppearancesProps) {
  const entryPeopleQuery = useEntryPeopleList(personId)
  const itemPeopleQuery = useItemPeopleList(personId)

  const entryPeople = entryPeopleQuery.data?.entryPeople ?? []
  const itemPeople = itemPeopleQuery.data?.itemPeople ?? []

  const libraryEntryIds = useMemo(() => entryPeople.map(row => row.libraryEntryId), [entryPeople])
  const itemIds = useMemo(() => itemPeople.map(row => row.itemId), [itemPeople])

  const { entriesById, isPending: entriesPending } = useLibraryEntriesByIds(libraryEntryIds)
  const { itemsById, isPending: itemsPending } = useItemsByIds(itemIds)

  // Doherty threshold — see docs/design/ux-principles.md#feedback--system-status,
  // same precedent PersonDetail's own GetPerson wait already set.
  if (entryPeopleQuery.isPending || itemPeopleQuery.isPending || entriesPending || itemsPending) {
    return null
  }

  if (entryPeopleQuery.isError || itemPeopleQuery.isError) {
    return (
      <p className="text-body text-status-failure" role="alert">
        Couldn't load appearances ({(entryPeopleQuery.error ?? itemPeopleQuery.error)?.message}).
      </p>
    )
  }

  const rows: AppearanceRow[] = [
    ...entryPeople
      .filter(row => entriesById[row.libraryEntryId])
      .map(row => ({
        key: `entry:${row.libraryEntryId}:${row.role}`,
        role: row.role,
        name: entriesById[row.libraryEntryId].name,
      })),
    ...itemPeople
      .filter(row => itemsById[row.itemId])
      .map(row => ({
        key: `item:${row.itemId}:${row.role}`,
        role: row.role,
        name: itemsById[row.itemId].title,
      })),
  ]

  return (
    <div className="flex max-w-sm flex-col gap-2 rounded-xl border border-border bg-surface p-4">
      <h2 className="text-title-md font-semibold text-text">Appears as</h2>

      {rows.length === 0 ? (
        <p className="text-body text-text-secondary">No known appearances yet.</p>
      ) : (
        <ul className="flex flex-col gap-2">
          {rows.map(row => (
            <li key={row.key} className="flex items-baseline gap-2">
              <span className="text-label text-text-secondary">{row.role}</span>
              <span className="text-body text-text">{row.name}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
