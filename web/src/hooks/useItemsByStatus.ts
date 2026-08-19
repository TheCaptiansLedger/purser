import { useInfiniteQuery } from '@connectrpc/connect-query'
import { listItems } from '../gen/purser/domain/v1/item-ItemService_connectquery'
import type { ItemStatus } from '../gen/purser/domain/v1/common_pb'

// WANTED_BOARD_PAGE_SIZE — the Wanted board's (#678) per-tab page size.
// Independent of any other page's own pageSize constant, same
// one-constant-per-call-site precedent as ARTIST_LIBRARY_PAGE_SIZE.
const WANTED_BOARD_PAGE_SIZE = 48

// useItemsByStatus wraps ItemService.ListItems, filtered to
// content_type="music" and the given status, as a cursor-paginated
// infinite query — one real server-side query per status bucket (#648's
// filter), never a client-side filter over one unfiltered load. Switching
// tabs on the Wanted board calls this with a different status and gets a
// fresh query, per this issue's own acceptance criterion.
export function useItemsByStatus(status: ItemStatus) {
  return useInfiniteQuery(
    listItems,
    { pageSize: WANTED_BOARD_PAGE_SIZE, pageToken: '', contentType: 'music', status },
    {
      pageParamKey: 'pageToken',
      getNextPageParam: lastPage => (lastPage.nextPageToken !== '' ? lastPage.nextPageToken : undefined),
    },
  )
}
