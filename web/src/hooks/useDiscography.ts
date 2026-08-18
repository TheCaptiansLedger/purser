import { createQueryOptions, useQuery, useTransport } from '@connectrpc/connect-query'
import { useQueries } from '@tanstack/react-query'
import { listGroups } from '../gen/purser/domain/v1/group-GroupService_connectquery'
import { listMusicReleases } from '../gen/purser/music/v1/release-MusicReleaseService_connectquery'
import { ReleaseStatus as ProtoReleaseStatus } from '../gen/purser/music/v1/release_pb'
import type { GroupRef, ReleaseStatus } from '../types'

// GROUPS_PAGE_SIZE — same "large enough that a real per-artist list never
// needs a second page" precedent as useEntryPeopleList's
// ENTRY_PEOPLE_PAGE_SIZE: this fan-out is bounded to one artist's groups
// (#667's own scope note — cheap, unlike the Library-grid-wide version in
// #670), so there's no "load more" control here.
const GROUPS_PAGE_SIZE = 200
const RELEASES_PAGE_SIZE = 50

// statusFromProto maps the wire ReleaseStatus enum to the plain string
// union MusicReleaseStatusBadge (#659) takes — RELEASE_STATUS_UNSPECIFIED
// has no case here because Release.Status is a required, validated oneof
// server-side (same reasoning as types/index.ts's ReleaseStatus comment).
function statusFromProto(status: ProtoReleaseStatus): ReleaseStatus | undefined {
  switch (status) {
    case ProtoReleaseStatus.STUB:
      return 'stub'
    case ProtoReleaseStatus.PARTIAL:
      return 'partial'
    case ProtoReleaseStatus.IMPORTED:
      return 'imported'
    default:
      return undefined
  }
}

// useDiscography — the Artist Detail Discography tab's (#667) read
// composition: GroupService.ListGroups(library_entry_id), then per group
// MusicReleaseService.ListMusicReleases(group_id) to derive the
// AlbumCard ownership badge. Group has no status of its own (ADR 0021);
// the badge is the default (is_default=true) edition's Status, falling
// back to the first returned edition when none is marked default, and
// left undefined when the Group has zero MusicRelease rows at all
// (AlbumCard's own "No edition selected" state handles that case — this
// hook just reports it rather than crashing on a missing default).
//
// useQueries + connect-query's createQueryOptions is the same bounded
// per-parent fan-out useLibraryEntriesByIds/usePersonImages already
// established — plain useQuery can't be called in a loop.
export function useDiscography(
  libraryEntryId: string,
): { albums: GroupRef[]; isPending: boolean; refetch: () => void } {
  const transport = useTransport()

  const groupsQuery = useQuery(
    listGroups,
    { pageSize: GROUPS_PAGE_SIZE, pageToken: '', libraryEntryId },
    { enabled: !!libraryEntryId },
  )
  const groups = groupsQuery.data?.groups ?? []

  const releaseResults = useQueries({
    queries: groups.map(group => ({
      ...createQueryOptions(
        listMusicReleases,
        { pageSize: RELEASES_PAGE_SIZE, pageToken: '', groupId: group.id, libraryEntryId: '' },
        { transport },
      ),
      staleTime: 5 * 60 * 1000,
      retry: false,
    })),
  })

  const albums: GroupRef[] = groups.map((group, index) => {
    const releases = releaseResults[index]?.data?.musicReleases ?? []
    const defaultRelease = releases.find(release => release.isDefault) ?? releases[0]
    return {
      id: group.id,
      title: group.title,
      year: group.year > 0 ? group.year : undefined,
      status: defaultRelease && statusFromProto(defaultRelease.status),
    }
  })

  return {
    albums,
    isPending: groupsQuery.isPending || releaseResults.some(result => result.isPending),
    // refetch — used by Add Album (#669) to bring the newly created Group
    // into view without navigating away: unlike Add Artist, there is no
    // Album Detail page yet (#673), so "done" means the new AlbumCard
    // just appears in this grid.
    refetch: () => void groupsQuery.refetch(),
  }
}
