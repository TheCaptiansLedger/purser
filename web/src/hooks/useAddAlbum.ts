import { ConnectError, Code } from '@connectrpc/connect'
import { useMutation } from '@connectrpc/connect-query'
import type { JsonObject } from '@bufbuild/protobuf'
import { useCallback } from 'react'
import { EntityType, MonitorMode } from '../gen/purser/domain/v1/common_pb'
import type { Group } from '../gen/purser/domain/v1/group_pb'
import {
  createExternalID,
  getExternalIDByValue,
} from '../gen/purser/domain/v1/external_id-ExternalIDService_connectquery'
import { createGroup, deleteGroup, getGroup } from '../gen/purser/domain/v1/group-GroupService_connectquery'
import type { MusicBrainzReleaseGroup } from '../gen/purser/music/v1/musicbrainz_search_pb'

const MBZ_SOURCE = 'mbz'

// albumType maps a MusicBrainzReleaseGroup onto ADR-0021's closed
// album_type vocabulary (studio|live|compilation|ep|single|other) for
// Group.Metadata — a direct port of internal/adapters/pipeline/music
// .albumType, kept in lockstep with that function. Live/Compilation are
// usually MusicBrainz secondary types layered on a primary type of
// Album, not primary types themselves, so secondary types are checked
// first.
export function albumType(releaseGroup: MusicBrainzReleaseGroup): string {
  for (const secondaryType of releaseGroup.secondaryTypes) {
    const lower = secondaryType.toLowerCase()
    if (lower === 'live') return 'live'
    if (lower === 'compilation') return 'compilation'
  }
  switch (releaseGroup.primaryType.toLowerCase()) {
    case 'album':
      return 'studio'
    case 'ep':
      return 'ep'
    case 'single':
      return 'single'
    case '':
      return ''
    default:
      return 'other'
  }
}

function isNotFound(err: unknown): boolean {
  return ConnectError.from(err).code === Code.NotFound
}

// useAddAlbum runs #669's Add Album get-or-create composition
// client-side, exactly ADR 0026's 3-step pattern useAddArtist (#665)
// already established, but for Group (entity_type=GROUP) with
// LibraryEntryID fixed to the current artist, and GroupService
// .CreateGroup in place of CreateLibraryEntry:
//
//  1. GetExternalIDByValue(GROUP, mbz, mbid) — hit → fetch and return
//     the existing album, done.
//  2. Miss → speculatively CreateGroup, fields mapped from the
//     MusicBrainzReleaseGroup DTO exactly per the backend Persister's own
//     getOrCreateReleaseGroup precedent (internal/adapters/pipeline
//     /music/persister.go): Title, Metadata.album_type — Year is
//     deliberately left unset, matching that function's own "Group.Year
//     is never populated" comment.
//  3. CreateExternalID linking it. A returned entity_id different from
//     the group just created means this caller lost the race — delete
//     the speculative group (safe, its fresh id nobody else could
//     reference yet) and use the winner instead.
//
// No enrichment steps here, unlike useAddArtist's TheAudioDB/MusicBrainz
// -relations best-effort steps — not in this issue's scope (see #669).
export function useAddAlbum() {
  const getExternalIDByValueMutation = useMutation(getExternalIDByValue)
  const getGroupMutation = useMutation(getGroup)
  const createGroupMutation = useMutation(createGroup)
  const createExternalIDMutation = useMutation(createExternalID)
  const deleteGroupMutation = useMutation(deleteGroup)

  const addAlbum = useCallback(
    async (candidate: MusicBrainzReleaseGroup, artistId: string): Promise<Group> => {
      // Step 1: look up by MBID first — the common, fast path.
      try {
        const hit = await getExternalIDByValueMutation.mutateAsync({
          entityType: EntityType.GROUP,
          source: MBZ_SOURCE,
          value: candidate.mbid,
        })
        const entityId = hit.externalId?.entityId
        if (entityId) {
          const existing = await getGroupMutation.mutateAsync({ id: entityId })
          if (existing.group) return existing.group
        }
      } catch (err) {
        if (!isNotFound(err)) throw err
      }

      // Step 2: speculative create.
      const metadata: JsonObject = {}
      const type = albumType(candidate)
      if (type !== '') metadata.album_type = type

      const created = await createGroupMutation.mutateAsync({
        group: {
          libraryEntryId: artistId,
          title: candidate.title,
          metadata,
          // Monitored/ALL is the same default-on-create useAddArtist uses —
          // MonitorMode is a required oneof on domain.Group.Validate
          // (MONITOR_MODE_UNSPECIFIED fails it), and an album a user just
          // chose to add is, by definition, one they want followed.
          monitored: true,
          monitorMode: MonitorMode.ALL,
        },
      })
      const createdGroup = created.group
      if (!createdGroup) throw new Error('CreateGroup returned no group')

      // Step 3: link the external id, then reconcile with whoever won.
      const linked = await createExternalIDMutation.mutateAsync({
        externalId: {
          entityType: EntityType.GROUP,
          entityId: createdGroup.id,
          source: MBZ_SOURCE,
          value: candidate.mbid,
        },
      })
      const winnerId = linked.externalId?.entityId

      if (winnerId && winnerId !== createdGroup.id) {
        await deleteGroupMutation.mutateAsync({ id: createdGroup.id, cascade: false })
        const winner = await getGroupMutation.mutateAsync({ id: winnerId })
        if (!winner.group) throw new Error('GetGroup returned no group for the winning id')
        return winner.group
      }

      return createdGroup
    },
    [getExternalIDByValueMutation, getGroupMutation, createGroupMutation, createExternalIDMutation, deleteGroupMutation],
  )

  return { addAlbum }
}
