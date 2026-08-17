import { createQueryOptions, useTransport } from '@connectrpc/connect-query'
import { useQueries } from '@tanstack/react-query'
import type { LibraryEntry } from '../gen/purser/domain/v1/library_entry_pb'
import { getLibraryEntry } from '../gen/purser/domain/v1/library_entry-LibraryEntryService_connectquery'

// useLibraryEntriesByIds resolves each given LibraryEntry id to its full
// LibraryEntry — the Person Detail page's (#661) "Appears as" section
// (#662) has only ids from ListEntryPeople, and needs each row's `name`
// to render. Same bounded per-page client-side fan-out usePersonImages
// already established (one Get per id via useQueries + connect-query's
// createQueryOptions — plain useQuery can't be called in a loop).
//
// Returns a libraryEntryId -> LibraryEntry map (omitting ids that failed
// to resolve rather than surfacing a partial/undefined entry) plus
// isPending, true while any Get is still in flight — the caller needs
// this to hold off rendering an incomplete row rather than a name that
// pops in after the fact.
export function useLibraryEntriesByIds(ids: string[]): { entriesById: Record<string, LibraryEntry>; isPending: boolean } {
  const transport = useTransport()

  const results = useQueries({
    queries: ids.map(id => ({
      ...createQueryOptions(getLibraryEntry, { id }, { transport }),
      staleTime: 5 * 60 * 1000,
      retry: false,
    })),
  })

  const entriesById: Record<string, LibraryEntry> = {}
  ids.forEach((id, index) => {
    const entry = results[index]?.data?.libraryEntry
    if (entry) {
      entriesById[id] = entry
    }
  })
  return { entriesById, isPending: results.some(result => result.isPending) }
}
