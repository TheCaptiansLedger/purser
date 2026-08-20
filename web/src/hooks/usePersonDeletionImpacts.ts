import { createQueryOptions, useTransport } from '@connectrpc/connect-query'
import { useQueries } from '@tanstack/react-query'
import { getPersonDeletionImpact } from '../gen/purser/domain/v1/person-PersonService_connectquery'
import { aggregateDeletionImpacts, type AggregatedImpactRow } from '../lib/deletionImpact'

// usePersonDeletionImpacts is the People index page's bulk-delete confirm-
// dialog data source: one GetPersonDeletionImpact fan-out per selected
// person — the same bounded useQueries fan-out useGroupDeletionImpacts/
// useLibraryEntryDeletionImpacts already established — summed by
// aggregateDeletionImpacts into one Kind-grouped list (see docs/adr/0015).
// Person never blocks a delete (see internal/service/person_deletion.go),
// so every row here is informational.
export function usePersonDeletionImpacts(ids: string[]): { isPending: boolean; rows: AggregatedImpactRow[] } {
  const transport = useTransport()

  const results = useQueries({
    queries: ids.map(id => ({
      ...createQueryOptions(getPersonDeletionImpact, { id }, { transport }),
      retry: false,
    })),
  })

  return {
    isPending: results.some(result => result.isPending),
    rows: aggregateDeletionImpacts(results.map(result => result.data?.impacts ?? [])),
  }
}
