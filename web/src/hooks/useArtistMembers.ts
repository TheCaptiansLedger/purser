import { useQuery } from '@connectrpc/connect-query'
import { timestampDate } from '@bufbuild/protobuf/wkt'
import type { Timestamp } from '@bufbuild/protobuf/wkt'
import { listEntryPeople } from '../gen/purser/domain/v1/entry_person-EntryPersonService_connectquery'
import { usePeopleByIds } from './usePeopleByIds'
import { usePersonImages } from './usePersonImages'

// ARTIST_MEMBERS_PAGE_SIZE — same "no load-more control on a per-entry
// tab" precedent as useDiscography's GROUPS_PAGE_SIZE/useEntryPeopleList's
// ENTRY_PEOPLE_PAGE_SIZE.
const ARTIST_MEMBERS_PAGE_SIZE = 200

// ArtistMemberRole is one EntryPerson row on this artist, resolved enough
// for both display (label, the raw role plus era suffix — PersonAppearances'
// own precedent, roles are never title-cased) and for #724's Edit/Remove
// actions, which need the row's own identity fields
// (personId/role/startDate/endDate — libraryEntryId is implied, this hook
// is always called for exactly one) to build UpdateEntryPerson/
// DeleteEntryPerson calls.
export interface ArtistMemberRole {
  role: string
  label: string
  startDate?: Timestamp
  endDate?: Timestamp
}

export interface ArtistMember {
  personId: string
  name: string
  imageId?: string
  roles: ArtistMemberRole[]
  former: boolean
}

// eraSuffix mirrors ArtistDetail's own facts-sidebar date formatting
// (start && end -> range, start only -> "since"). getUTCFullYear, not
// getFullYear: a UTC midnight timestamp (e.g. a bare "1975" date stored
// as 1975-01-01T00:00:00Z) reads back as the previous year in any
// negative-UTC-offset local timezone otherwise.
function eraSuffix(startDate?: Timestamp, endDate?: Timestamp): string {
  const start = startDate && timestampDate(startDate).getUTCFullYear()
  const end = endDate && timestampDate(endDate).getUTCFullYear()
  if (start && end) return ` (${start}–${end})`
  if (start) return ` (since ${start})`
  return ''
}

// useArtistMembers — the Artist Detail Members tab's (#668, #724) read
// composition: EntryPersonService.ListEntryPeople(library_entry_id), each
// row resolved to its Person (usePeopleByIds) and selected photo
// (usePersonImages), grouped by person into one ArtistMember per person
// carrying every role row they hold on this entry — #724's Edit/Remove
// row list flattens `roles` back out one row per (person, role) at render
// time; the grouping stays here only to derive `former` and to dedupe the
// Person/image fan-out per person, not per row.
//
// "Former" is per-entity, not global (a person can be a current member of
// one artist and a former member of another): a person counts as former
// here only when every one of *their* rows on *this* library entry
// carries an EndDate — any still-open row makes them current, even if
// they also hold a separate, already-ended role.
export function useArtistMembers(
  libraryEntryId: string,
): { members: ArtistMember[]; isPending: boolean; isError: boolean; refetch: () => void } {
  const entryPeopleQuery = useQuery(
    listEntryPeople,
    { libraryEntryId, personId: '', pageSize: ARTIST_MEMBERS_PAGE_SIZE, pageToken: '' },
    { enabled: !!libraryEntryId },
  )
  const rows = entryPeopleQuery.data?.entryPeople ?? []

  const personIds = [...new Set(rows.map(row => row.personId))]
  const { peopleById, isPending: peoplePending } = usePeopleByIds(personIds)
  const imagesByPersonId = usePersonImages(personIds)

  const byPerson = new Map<string, { roles: ArtistMemberRole[]; former: boolean }>()
  for (const row of rows) {
    const entry = byPerson.get(row.personId) ?? { roles: [], former: true }
    entry.roles.push({
      role: row.role,
      label: `${row.role}${eraSuffix(row.startDate, row.endDate)}`,
      startDate: row.startDate,
      endDate: row.endDate,
    })
    if (!row.endDate) entry.former = false
    byPerson.set(row.personId, entry)
  }

  const members: ArtistMember[] = [...byPerson.entries()]
    .filter(([personId]) => peopleById[personId])
    .map(([personId, { roles, former }]) => ({
      personId,
      name: peopleById[personId].name,
      imageId: imagesByPersonId[personId],
      roles,
      former,
    }))

  return {
    members,
    isPending: entryPeopleQuery.isPending || peoplePending,
    isError: entryPeopleQuery.isError,
    refetch: () => void entryPeopleQuery.refetch(),
  }
}
