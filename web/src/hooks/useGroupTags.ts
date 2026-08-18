import { createQueryOptions, useQuery, useTransport } from '@connectrpc/connect-query'
import { useQueries } from '@tanstack/react-query'
import { EntityType } from '../gen/purser/domain/v1/common_pb'
import { getTag } from '../gen/purser/domain/v1/tag-TagService_connectquery'
import { listTagAssignments } from '../gen/purser/domain/v1/tag_assignment-TagAssignmentService_connectquery'

// GROUP_TAG_ASSIGNMENTS_PAGE_SIZE — same "no load-more control on a
// per-entity chip row" precedent as useArtistMembers'
// ARTIST_MEMBERS_PAGE_SIZE/useDiscography's GROUPS_PAGE_SIZE: one album's
// genre/mood tag count is small, bounded, no pagination concern (#673's
// own scope note).
const GROUP_TAG_ASSIGNMENTS_PAGE_SIZE = 200

export interface GroupTag {
  id: string
  key: string
  value: string
}

// useGroupTags — the Album Detail genre/mood chip row's (#673) read
// composition: TagAssignmentService.ListTagAssignments(entity_type=GROUP,
// entity_id) then TagService.GetTag per assignment, same bounded
// per-entity fan-out useArtistMembers/usePeopleByIds already established
// (one Get per id via useQueries + connect-query's createQueryOptions —
// plain useQuery can't be called in a loop).
//
// Returns whatever Tags resolve (an assignment whose Tag failed to
// resolve is simply omitted, not surfaced as a page-level error) so a
// Group with zero tag assignments renders an empty array — the caller
// omits the chip row entirely rather than rendering an empty placeholder
// (#673's acceptance criterion).
export function useGroupTags(groupId: string): { tags: GroupTag[]; isPending: boolean } {
  const transport = useTransport()

  const assignmentsQuery = useQuery(
    listTagAssignments,
    { tagId: '', entityType: EntityType.GROUP, entityId: groupId, pageSize: GROUP_TAG_ASSIGNMENTS_PAGE_SIZE, pageToken: '' },
    { enabled: !!groupId },
  )
  const assignments = assignmentsQuery.data?.tagAssignments ?? []

  const results = useQueries({
    queries: assignments.map(assignment => ({
      ...createQueryOptions(getTag, { id: assignment.tagId }, { transport }),
      staleTime: 5 * 60 * 1000,
      retry: false,
    })),
  })

  const tags: GroupTag[] = []
  results.forEach(result => {
    const tag = result.data?.tag
    if (tag) {
      tags.push({ id: tag.id, key: tag.key, value: tag.value })
    }
  })

  return {
    tags,
    isPending: assignmentsQuery.isPending || results.some(result => result.isPending),
  }
}
