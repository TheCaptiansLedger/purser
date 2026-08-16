import { useInfiniteQuery } from '@connectrpc/connect-query'
import { listPeople } from '../gen/purser/domain/v1/person-PersonService_connectquery'

// PEOPLE_PAGE_SIZE — the People index page's (#660) grid page size.
// Independent of any other page's own pageSize constant, same
// one-constant-per-call-site precedent as useJobsList's JOBS_PAGE_SIZE.
const PEOPLE_PAGE_SIZE = 48

// usePeopleList wraps PersonService.ListPeople as a cursor-paginated
// infinite query — the People index page's (#660) "load more" grid. name
// is the search box's query, sent server-side via #653's ListPeopleRequest
// filter rather than filtered client-side against whatever page happens to
// be loaded.
export function usePeopleList(name: string) {
  return useInfiniteQuery(
    listPeople,
    { pageSize: PEOPLE_PAGE_SIZE, pageToken: '', name },
    {
      pageParamKey: 'pageToken',
      getNextPageParam: lastPage => (lastPage.nextPageToken !== '' ? lastPage.nextPageToken : undefined),
    },
  )
}
