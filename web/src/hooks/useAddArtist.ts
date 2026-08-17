import { ConnectError, Code } from '@connectrpc/connect'
import { useMutation } from '@connectrpc/connect-query'
import type { JsonObject } from '@bufbuild/protobuf'
import { timestampFromDate } from '@bufbuild/protobuf/wkt'
import { useCallback } from 'react'
import { EntityType, MonitorMode } from '../gen/purser/domain/v1/common_pb'
import type { LibraryEntry } from '../gen/purser/domain/v1/library_entry_pb'
import type { MusicBrainzArtist, MusicBrainzArtistMember } from '../gen/purser/music/v1/musicbrainz_search_pb'
import {
  createExternalID,
  getExternalIDByValue,
} from '../gen/purser/domain/v1/external_id-ExternalIDService_connectquery'
import { createEntryPerson } from '../gen/purser/domain/v1/entry_person-EntryPersonService_connectquery'
import {
  createLibraryEntry,
  deleteLibraryEntry,
  getLibraryEntry,
  updateLibraryEntry,
} from '../gen/purser/domain/v1/library_entry-LibraryEntryService_connectquery'
import { createPerson } from '../gen/purser/domain/v1/person-PersonService_connectquery'
import { Gender } from '../gen/purser/domain/v1/person_pb'
import { getArtist } from '../gen/purser/music/v1/musicbrainz_search-MusicBrainzService_connectquery'
import { lookupArtist } from '../gen/purser/music/v1/theaudiodb-TheAudioDBService_connectquery'

const MBZ_SOURCE = 'mbz'

// libraryEntryMetadataFromMusicBrainzArtist maps MusicBrainzArtist onto
// LibraryEntry.Metadata exactly per ADR 0021's reuse table — artist_type,
// aliases, country, and founded_date/dissolved_date (or born_date/died_date
// for a solo "Person") from life_span_begin/life_span_end. isni/
// official_url/wikipedia_url aren't on this DTO (see musicbrainz_search
// .proto's own doc comment — that data lives on GetArtist's response, not
// the search-picker DTO) — those are merged in separately, by
// useAddArtist's own best-effort MusicBrainz-relations step below. Empty
// strings/arrays are omitted rather than written as empty values.
export function libraryEntryMetadataFromMusicBrainzArtist(artist: MusicBrainzArtist): JsonObject {
  const metadata: JsonObject = {}
  if (artist.type !== '') metadata.artist_type = artist.type
  if (artist.country !== '') metadata.country = artist.country
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

// musicBrainzMemberDate converts a MusicBrainz relation's partial date
// string ("1972", "1972-11", "1972-11-07") into a Timestamp — `new Date()`
// parses all three ISO-8601-prefix forms directly, so no hand-rolled
// parser is needed. Empty string (MusicBrainz's own "unknown" shape for
// begin/end) returns undefined rather than an invented epoch date.
function musicBrainzMemberDate(value: string) {
  if (value === '') return undefined
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? undefined : timestampFromDate(date)
}

// memberRole derives EntryPerson.Role from a MusicBrainz "member of band"
// relation's own attributes (its instrument/contribution list, e.g.
// "vocal", "guitar") — role vocabulary is adapter/UI knowledge per ADR
// 0001 rule 3, so this mapping belongs here, not in shared domain code.
// A relation with no attributes still needs some non-empty Role
// (domain.EntryPerson.Validate requires one) — "Member" is the fallback.
function memberRole(member: MusicBrainzArtistMember): string {
  return member.attributes.length > 0 ? member.attributes.join(', ') : 'Member'
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
// Then, two independent best-effort enrichment steps — either failing
// (a provider miss, a network error) is swallowed and never fails the
// flow, per this issue's scope note:
//
//  - TheAudioDBService.LookupArtist(mbid): a hit writes Genre/Style into
//    Metadata["genre"]/["style"].
//  - MusicBrainzService.GetArtist(mbid): a hit writes Isni/OfficialUrl/
//    WikipediaUrl into Metadata, and — for a Type=="Group" artist only —
//    get-or-creates a Person (ADR 0026 pattern, source "mbz", same 3-step
//    shape as the LibraryEntry get-or-create above) and
//    EntryPersonService.CreateEntryPerson for every band-member relation
//    MusicBrainz returns. A Type=="Person" (solo) artist gets none of
//    this — that's #672's own linking step, a human-driven choice, not
//    an automatic one.
export function useAddArtist() {
  const getExternalIDByValueMutation = useMutation(getExternalIDByValue)
  const getLibraryEntryMutation = useMutation(getLibraryEntry)
  const createLibraryEntryMutation = useMutation(createLibraryEntry)
  const createExternalIDMutation = useMutation(createExternalID)
  const deleteLibraryEntryMutation = useMutation(deleteLibraryEntry)
  const updateLibraryEntryMutation = useMutation(updateLibraryEntry)
  const lookupArtistMutation = useMutation(lookupArtist)
  const getArtistMutation = useMutation(getArtist)
  const createPersonMutation = useMutation(createPerson)
  const createEntryPersonMutation = useMutation(createEntryPerson)

  // getOrCreatePerson mirrors the LibraryEntry get-or-create above (ADR
  // 0026, same 3-step shape) for a MusicBrainz band member: hit → reuse
  // the existing Person; miss → speculative CreatePerson, then
  // CreateExternalID to link it, reconciling with whoever won the race.
  // Gender.UNKNOWN is the only honest default — MusicBrainz's artist
  // relations carry no gender for a band member, same "leave it
  // unguessed" rule PersonDialog's own create defaults already use.
  const getOrCreatePerson = useCallback(
    async (member: MusicBrainzArtistMember): Promise<string> => {
      try {
        const hit = await getExternalIDByValueMutation.mutateAsync({
          entityType: EntityType.PERSON,
          source: MBZ_SOURCE,
          value: member.mbid,
        })
        const entityId = hit.externalId?.entityId
        if (entityId) return entityId
      } catch (err) {
        if (!isNotFound(err)) throw err
      }

      const created = await createPersonMutation.mutateAsync({
        person: {
          name: member.name,
          sortName: member.name,
          gender: Gender.UNKNOWN,
          // MonitorMode is a required oneof on domain.Person.Validate
          // (MONITOR_MODE_UNSPECIFIED fails it) — same regression
          // PersonDialog's own create defaults already guard against.
          monitored: true,
          monitorMode: MonitorMode.ALL,
        },
      })
      const createdPerson = created.person
      if (!createdPerson) throw new Error('CreatePerson returned no person')

      const linked = await createExternalIDMutation.mutateAsync({
        externalId: { entityType: EntityType.PERSON, entityId: createdPerson.id, source: MBZ_SOURCE, value: member.mbid },
      })
      const winnerId = linked.externalId?.entityId
      return winnerId && winnerId !== createdPerson.id ? winnerId : createdPerson.id
    },
    [getExternalIDByValueMutation, createPersonMutation, createExternalIDMutation],
  )

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

      // Best-effort MusicBrainz-relations enrichment: ISNI/official/
      // wikipedia links into Metadata, plus (Group artists only)
      // band-member Person/EntryPerson creation. Never fails the flow.
      try {
        const relations = await getArtistMutation.mutateAsync({ mbid: candidate.mbid })

        const isni = relations.isnis[0]
        if (isni || relations.officialUrl !== '' || relations.wikipediaUrl !== '') {
          const mergedMetadata: JsonObject = { ...(entry.metadata ?? {}) }
          if (isni) mergedMetadata.isni = isni
          if (relations.officialUrl !== '') mergedMetadata.official_url = relations.officialUrl
          if (relations.wikipediaUrl !== '') mergedMetadata.wikipedia_url = relations.wikipediaUrl

          const updated = await updateLibraryEntryMutation.mutateAsync({
            libraryEntry: { id: entry.id, metadata: mergedMetadata },
            updateMask: { paths: ['metadata'] },
          })
          if (updated.libraryEntry) entry = updated.libraryEntry
        }

        if (candidate.type === 'Group') {
          for (const member of relations.members) {
            try {
              const personId = await getOrCreatePerson(member)
              await createEntryPersonMutation.mutateAsync({
                entryPerson: {
                  libraryEntryId: entry.id,
                  personId,
                  role: memberRole(member),
                  startDate: musicBrainzMemberDate(member.begin),
                  endDate: musicBrainzMemberDate(member.end),
                },
              })
            } catch (err) {
              // A CreateEntryPerson conflict (role already linked, e.g. a
              // re-run against an already-imported artist) or a single
              // member's own Person creation failing must not abort the
              // rest of the roster — each member is independent.
              if (!isNotFound(err) && ConnectError.from(err).code !== Code.AlreadyExists) throw err
            }
          }
        }
      } catch {
        // MusicBrainz has no relations data for this mbid, or the lookup
        // failed — best-effort only, same as the TheAudioDB step above.
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
      getArtistMutation,
      getOrCreatePerson,
      createEntryPersonMutation,
    ],
  )

  return { addArtist }
}
