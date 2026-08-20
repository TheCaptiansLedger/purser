import { createQueryOptions, useTransport } from '@connectrpc/connect-query'
import { useQueries } from '@tanstack/react-query'
import { getGroupDeletionImpact } from '../gen/purser/domain/v1/group-GroupService_connectquery'
import { aggregateDeletionImpacts, type AggregatedImpactRow } from '../lib/deletionImpact'

// useGroupDeletionImpacts is the Discography tab's bulk-delete (#679)
// confirm-dialog data source: one GetGroupDeletionImpact fan-out per
// selected album — the same bounded useQueries fan-out useDiscography
// already established for this tab — summed by aggregateDeletionImpacts
// into one Kind-grouped list (see docs/adr/0015). Group never blocks a
// delete (see internal/service/group_deletion.go), so every row here is
// informational, unlike useLibraryEntryDeletionImpacts' Groups/Items rows.
export function useGroupDeletionImpacts(ids: string[]): { isPending: boolean; rows: AggregatedImpactRow[] } {
  const transport = useTransport()

  const results = useQueries({
    queries: ids.map(id => ({
      ...createQueryOptions(getGroupDeletionImpact, { id }, { transport }),
      retry: false,
    })),
  })

  return {
    isPending: results.some(result => result.isPending),
    rows: aggregateDeletionImpacts(results.map(result => result.data?.impacts ?? [])),
  }
}
