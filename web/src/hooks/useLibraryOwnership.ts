import { createQueryOptions, useTransport } from '@connectrpc/connect-query'
import { useQueries } from '@tanstack/react-query'
import { listGroups } from '../gen/purser/domain/v1/group-GroupService_connectquery'
import { listMusicReleases } from '../gen/purser/music/v1/release-MusicReleaseService_connectquery'
import { ReleaseStatus as ProtoReleaseStatus } from '../gen/purser/music/v1/release_pb'
import type { OwnershipRingProps } from '../components/OwnershipRing'

// GROUPS_PAGE_SIZE/RELEASES_PAGE_SIZE — same values useDiscography.ts
// uses, kept as this call site's own constants rather than a shared
// import, per that file's own "one constant per call site" precedent.
const GROUPS_PAGE_SIZE = 200
const RELEASES_PAGE_SIZE = 50

// useLibraryOwnership — the Music Library grid's (#670) ownership-ring
// fan-out: GroupService.ListGroups(library_entry_id) per artist on the
// loaded page, then MusicReleaseService.ListMusicReleases(group_id) per
// group, to derive "N of M albums owned" (an album counts as owned when
// its default edition's Status is IMPORTED — same "owned" reading
// AlbumCard/useDiscography give that status). Group has no status of its
// own (ADR 0021), hence the fan-out — see docs/technical/music-web-ui.md's
// "Discography ownership" section, which explicitly accepts this as a
// bounded, per-page client-side cost rather than a new aggregate RPC.
//
// Cancellation: no hand-rolled AbortController. connect-query's queryFn
// forwards TanStack Query's own per-query AbortSignal into the RPC call,
// so a query that becomes unobserved (component unmount on navigate-away)
// has its in-flight request aborted automatically — the same mechanism
// useDiscography/useLibraryEntryImages already rely on without stating it
// explicitly; called out here because #670's acceptance criteria name it.
//
// Every input id always gets an entry in the returned map (isPending
// starts true), and each artist's groups/releases queries are independent
// per-id useQueries entries — one artist's ListGroups/ListMusicReleases
// erroring (e.g. a malformed record) only marks that artist's own entry
// isError, never blocks or blanks the rest of the grid.
export function useLibraryOwnership(libraryEntryIds: string[]): Record<string, OwnershipRingProps> {
  const transport = useTransport()

  const groupsResults = useQueries({
    queries: libraryEntryIds.map(libraryEntryId => ({
      ...createQueryOptions(
        listGroups,
        { pageSize: GROUPS_PAGE_SIZE, pageToken: '', libraryEntryId },
        { transport },
      ),
      staleTime: 5 * 60 * 1000,
      retry: false,
    })),
  })

  // Flatten every artist's groups into one array so useQueries stays a
  // single flat list (its own requirement) — groupEntries.length tracks
  // groupsResults' resolved data 1:1 with releaseResults below.
  const groupEntries = libraryEntryIds.flatMap((_, index) => groupsResults[index]?.data?.groups ?? [])

  const releaseResults = useQueries({
    queries: groupEntries.map(group => ({
      ...createQueryOptions(
        listMusicReleases,
        { pageSize: RELEASES_PAGE_SIZE, pageToken: '', groupId: group.id, libraryEntryId: '' },
        { transport },
      ),
      staleTime: 5 * 60 * 1000,
      retry: false,
    })),
  })

  const ownershipByLibraryEntryId: Record<string, OwnershipRingProps> = {}
  let offset = 0
  libraryEntryIds.forEach((libraryEntryId, index) => {
    const groupsQuery = groupsResults[index]
    const groups = groupsQuery?.data?.groups ?? []
    const releasesForArtist = releaseResults.slice(offset, offset + groups.length)
    offset += groups.length

    const owned = releasesForArtist.filter(result => {
      const releases = result?.data?.musicReleases ?? []
      const defaultRelease = releases.find(release => release.isDefault) ?? releases[0]
      return defaultRelease?.status === ProtoReleaseStatus.IMPORTED
    }).length

    ownershipByLibraryEntryId[libraryEntryId] = {
      owned,
      total: groups.length,
      isPending: groupsQuery?.isPending ?? false,
      isError: (groupsQuery?.isError ?? false) || releasesForArtist.some(result => result?.isError),
    }
  })

  return ownershipByLibraryEntryId
}
