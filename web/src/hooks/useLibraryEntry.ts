import { useMutation, useQuery } from '@connectrpc/connect-query'
import {
  getLibraryEntry,
  updateLibraryEntry,
} from '../gen/purser/domain/v1/library_entry-LibraryEntryService_connectquery'

// useLibraryEntry wraps LibraryEntryService.GetLibraryEntry — the Artist
// Detail page's (#666) one read. enabled: !!id, same usePerson.ts
// precedent since the route param is technically optional to the type
// system even though the route always supplies one.
export function useLibraryEntry(id: string) {
  return useQuery(getLibraryEntry, { id }, { enabled: !!id })
}

// useUpdateLibraryEntryMutation is a separate hook, not folded into
// useLibraryEntry, so a component only subscribes to the mutation state
// it actually renders — same one-hook-per-RPC shape usePerson.ts already
// established. Does not auto-invalidate useLibraryEntry's query cache;
// callers reconcile their own optimistic state from the mutation's
// response instead (round-trip confirmation, not a refetch).
export function useUpdateLibraryEntryMutation() {
  return useMutation(updateLibraryEntry)
}
