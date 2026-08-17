import { ConnectError, Code } from '@connectrpc/connect'
import { useMutation } from '@connectrpc/connect-query'
import type { JsonObject } from '@bufbuild/protobuf'
import { useCallback } from 'react'
import { EntityType, MonitorMode } from '../gen/purser/domain/v1/common_pb'
import type { LibraryEntry } from '../gen/purser/domain/v1/library_entry_pb'
import type { MusicBrainzArtist } from '../gen/purser/music/v1/musicbrainz_search_pb'
import {
  createExternalID,
  getExternalIDByValue,
} from '../gen/purser/domain/v1/external_id-ExternalIDService_connectquery'
import {
  createLibraryEntry,
  deleteLibraryEntry,
  getLibraryEntry,
  updateLibraryEntry,
} from '../gen/purser/domain/v1/library_entry-LibraryEntryService_connectquery'
import { lookupArtist } from '../gen/purser/music/v1/theaudiodb-TheAudioDBService_connectquery'

const MBZ_SOURCE = 'mbz'

// libraryEntryMetadataFromMusicBrainzArtist maps MusicBrainzArtist onto
// LibraryEntry.Metadata exactly per ADR 0021's reuse table — artist_type,
// aliases, and founded_date/dissolved_date (or born_date/died_date for a
// solo "Person") from life_span_begin/life_span_end. isni/official_url/
// wikipedia_url aren't on this DTO (see musicbrainz_search.proto's own
// doc comment) so they're never invented here — left unset, same as any
// other field this DTO doesn't carry. Empty strings/arrays are omitted
// rather than written as empty values.
export function libraryEntryMetadataFromMusicBrainzArtist(artist: MusicBrainzArtist): JsonObject {
  const metadata: JsonObject = {}
  if (artist.type !== '') metadata.artist_type = artist.type
  if (artist.aliases.length > 0) metadata.aliases = artist.aliases

  const isPerson = artist.type === 'Person'
  const beginKey = isPerson ? 'born_date' : 'founded_date'
  const endKey = isPerson ? 'died_date' : 'dissolved_date'
  if (artist.lifeSpanBegin !== '') metadata[beginKey] = artist.lifeSpanBegin
  if (artist.lifeSpanEnd !== '') metadata[endKey] = artist.lifeSpanEnd

  return metadata
}

function isNotFound(err: unknown): boolean {
  return ConnectError.from(err).code === Code.NotFound
}

// useAddArtist runs #665's Add Artist get-or-create composition
// client-side, exactly ADR 0026's 3-step pattern:
//
//  1. GetExternalIDByValue(LIBRARY_ENTRY, mbz, mbid) — hit → fetch and
//     return the existing artist, done.
//  2. Miss → speculatively CreateLibraryEntry from the MusicBrainzArtist
//     DTO.
//  3. CreateExternalID linking it. A returned entity_id different from
//     the entry just created means this caller lost the race — delete
//     the speculative entry (safe, its fresh id nobody else could
//     reference yet) and use the winner instead.
//
// Then, best-effort: TheAudioDBService.LookupArtist(mbid) — a hit writes
// Genre/Style into Metadata["genre"]/["style"] via a field-masked
// UpdateLibraryEntry; a miss (or any other error) is swallowed, per this
// issue's scope note — it never fails the flow.
export function useAddArtist() {
  const getExternalIDByValueMutation = useMutation(getExternalIDByValue)
  const getLibraryEntryMutation = useMutation(getLibraryEntry)
  const createLibraryEntryMutation = useMutation(createLibraryEntry)
  const createExternalIDMutation = useMutation(createExternalID)
  const deleteLibraryEntryMutation = useMutation(deleteLibraryEntry)
  const updateLibraryEntryMutation = useMutation(updateLibraryEntry)
  const lookupArtistMutation = useMutation(lookupArtist)

  const addArtist = useCallback(
    async (candidate: MusicBrainzArtist): Promise<LibraryEntry> => {
      // Step 1: look up by MBID first — the common, fast path.
      try {
        const hit = await getExternalIDByValueMutation.mutateAsync({
          entityType: EntityType.LIBRARY_ENTRY,
          source: MBZ_SOURCE,
          value: candidate.mbid,
        })
        const entityId = hit.externalId?.entityId
        if (entityId) {
          const existing = await getLibraryEntryMutation.mutateAsync({ id: entityId })
          if (existing.libraryEntry) return existing.libraryEntry
        }
      } catch (err) {
        if (!isNotFound(err)) throw err
      }

      // Step 2: speculative create.
      const created = await createLibraryEntryMutation.mutateAsync({
        libraryEntry: {
          contentType: 'music',
          kind: 'artist',
          name: candidate.name,
          sortName: candidate.sortName,
          metadata: libraryEntryMetadataFromMusicBrainzArtist(candidate),
          // Monitored/ALL is the same default-on-create PersonDialog uses —
          // MonitorMode is a required oneof on domain.LibraryEntry.Validate
          // (MONITOR_MODE_UNSPECIFIED fails it), and an artist a user just
          // chose to add is, by definition, one they want followed.
          monitored: true,
          monitorMode: MonitorMode.ALL,
        },
      })
      const createdEntry = created.libraryEntry
      if (!createdEntry) throw new Error('CreateLibraryEntry returned no library entry')

      // Step 3: link the external id, then reconcile with whoever won.
      const linked = await createExternalIDMutation.mutateAsync({
        externalId: {
          entityType: EntityType.LIBRARY_ENTRY,
          entityId: createdEntry.id,
          source: MBZ_SOURCE,
          value: candidate.mbid,
        },
      })
      const winnerId = linked.externalId?.entityId

      let entry = createdEntry
      if (winnerId && winnerId !== createdEntry.id) {
        await deleteLibraryEntryMutation.mutateAsync({ id: createdEntry.id, cascade: false })
        const winner = await getLibraryEntryMutation.mutateAsync({ id: winnerId })
        if (!winner.libraryEntry) throw new Error('GetLibraryEntry returned no library entry for the winning id')
        entry = winner.libraryEntry
      }

      // Best-effort TheAudioDB genre/style enrichment — never fails the flow.
      try {
        const lookup = await lookupArtistMutation.mutateAsync({ mbid: candidate.mbid })
        const tadbArtist = lookup.artist
        if (tadbArtist && (tadbArtist.genre !== '' || tadbArtist.style !== '')) {
          const mergedMetadata: JsonObject = { ...(entry.metadata ?? {}) }
          if (tadbArtist.genre !== '') mergedMetadata.genre = tadbArtist.genre
          if (tadbArtist.style !== '') mergedMetadata.style = tadbArtist.style

          const updated = await updateLibraryEntryMutation.mutateAsync({
            libraryEntry: { id: entry.id, metadata: mergedMetadata },
            updateMask: { paths: ['metadata'] },
          })
          if (updated.libraryEntry) entry = updated.libraryEntry
        }
      } catch {
        // TheAudioDB has no match, or the lookup failed — best-effort only.
      }

      return entry
    },
    [
      getExternalIDByValueMutation,
      getLibraryEntryMutation,
      createLibraryEntryMutation,
      createExternalIDMutation,
      deleteLibraryEntryMutation,
      updateLibraryEntryMutation,
      lookupArtistMutation,
    ],
  )

  return { addArtist }
}
