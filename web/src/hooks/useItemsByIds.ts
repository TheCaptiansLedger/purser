import { createQueryOptions, useTransport } from '@connectrpc/connect-query'
import { useQueries } from '@tanstack/react-query'
import type { Item } from '../gen/purser/domain/v1/item_pb'
import { getItem } from '../gen/purser/domain/v1/item-ItemService_connectquery'

// useItemsByIds resolves each given Item id to its full Item — same
// reasoning and shape as useLibraryEntriesByIds, the Item-credit half of
// the Person Detail page's (#661) "Appears as" section (#662).
//
// Returns an itemId -> Item map (omitting ids that failed to resolve)
// plus isPending, true while any Get is still in flight.
export function useItemsByIds(ids: string[]): { itemsById: Record<string, Item>; isPending: boolean } {
  const transport = useTransport()

  const results = useQueries({
    queries: ids.map(id => ({
      ...createQueryOptions(getItem, { id }, { transport }),
      staleTime: 5 * 60 * 1000,
      retry: false,
    })),
  })

  const itemsById: Record<string, Item> = {}
  ids.forEach((id, index) => {
    const item = results[index]?.data?.item
    if (item) {
      itemsById[id] = item
    }
  })
  return { itemsById, isPending: results.some(result => result.isPending) }
}
