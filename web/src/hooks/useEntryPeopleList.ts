import { useQuery } from '@connectrpc/connect-query'
import { listEntryPeople } from '../gen/purser/domain/v1/entry_person-EntryPersonService_connectquery'

// ENTRY_PEOPLE_PAGE_SIZE — large enough that a Person Detail page's
// (#661) "Appears as" section (#662) never needs a second page in
// practice; unlike People/Jobs this section has no "load more" control,
// so pagination beyond the first page isn't handled here.
const ENTRY_PEOPLE_PAGE_SIZE = 200

// useEntryPeopleList wraps EntryPersonService.ListEntryPeople, filtered
// to one person across every LibraryEntry they're credited on
// (libraryEntryId left empty — that filter is for the opposite
// direction, an entry's cast/crew list, not built yet).
export function useEntryPeopleList(personId: string) {
  return useQuery(
    listEntryPeople,
    { libraryEntryId: '', personId, pageSize: ENTRY_PEOPLE_PAGE_SIZE, pageToken: '' },
    { enabled: !!personId },
  )
}
