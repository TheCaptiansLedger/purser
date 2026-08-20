import { createQueryOptions, useTransport } from '@connectrpc/connect-query'
import { useQueries } from '@tanstack/react-query'
import { getLibraryEntryDeletionImpact } from '../gen/purser/domain/v1/library_entry-LibraryEntryService_connectquery'
import { aggregateDeletionImpacts, type AggregatedImpactRow } from '../lib/deletionImpact'

// useLibraryEntryDeletionImpacts is the Music Library grid's bulk-delete
// (#679) confirm-dialog data source: one GetLibraryEntryDeletionImpact
// fan-out per selected artist — the same bounded useQueries fan-out
// useLibraryOwnership/useLibraryEntryImages already established for this
// page — summed by aggregateDeletionImpacts into one Kind-grouped list
// (see docs/adr/0015). Bounded to the current selection, never the whole
// library.
export function useLibraryEntryDeletionImpacts(ids: string[]): { isPending: boolean; rows: AggregatedImpactRow[] } {
  const transport = useTransport()

  const results = useQueries({
    queries: ids.map(id => ({
      ...createQueryOptions(getLibraryEntryDeletionImpact, { id }, { transport }),
      retry: false,
    })),
  })

  return {
    isPending: results.some(result => result.isPending),
    rows: aggregateDeletionImpacts(results.map(result => result.data?.impacts ?? [])),
  }
}
