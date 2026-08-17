import { useQuery } from '@connectrpc/connect-query'
import { listItemPeople } from '../gen/purser/domain/v1/item_person-ItemPersonService_connectquery'

// ITEM_PEOPLE_PAGE_SIZE — same reasoning as useEntryPeopleList's
// ENTRY_PEOPLE_PAGE_SIZE: no "load more" control on the Person Detail
// page's (#661) "Appears as" section (#662).
const ITEM_PEOPLE_PAGE_SIZE = 200

// useItemPeopleList wraps ItemPersonService.ListItemPeople, filtered to
// one person across every Item they're credited on (itemId left empty —
// that filter is for the opposite direction, an item's cast/crew list,
// not built yet).
export function useItemPeopleList(personId: string) {
  return useQuery(
    listItemPeople,
    { itemId: '', personId, pageSize: ITEM_PEOPLE_PAGE_SIZE, pageToken: '' },
    { enabled: !!personId },
  )
}
