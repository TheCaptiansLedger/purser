import { createQueryOptions, useTransport } from '@connectrpc/connect-query'
import { useQueries } from '@tanstack/react-query'
import type { Group } from '../gen/purser/domain/v1/group_pb'
import { getGroup } from '../gen/purser/domain/v1/group-GroupService_connectquery'

// useGroupsByIds resolves each given Group id to its full Group — same
// shape and reasoning as useLibraryEntriesByIds/useItemsByIds: a bounded
// per-page fan-out (one Get per id via useQueries + connect-query's
// createQueryOptions) rather than a new aggregate RPC. The Wanted board
// (#678) is the first caller, resolving each Item's group_id to its
// album title.
//
// Returns a groupId -> Group map (omitting ids that failed to resolve)
// plus isPending, true while any Get is still in flight.
export function useGroupsByIds(ids: string[]): { groupsById: Record<string, Group>; isPending: boolean } {
  const transport = useTransport()

  const results = useQueries({
    queries: ids.map(id => ({
      ...createQueryOptions(getGroup, { id }, { transport }),
      staleTime: 5 * 60 * 1000,
      retry: false,
    })),
  })

  const groupsById: Record<string, Group> = {}
  ids.forEach((id, index) => {
    const group = results[index]?.data?.group
    if (group) {
      groupsById[id] = group
    }
  })
  return { groupsById, isPending: results.some(result => result.isPending) }
}
