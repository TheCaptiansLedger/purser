import { ConnectError, Code } from '@connectrpc/connect'
import { useMutation } from '@connectrpc/connect-query'
import type { Timestamp } from '@bufbuild/protobuf/wkt'
import { useCallback } from 'react'
import { EntityType, MonitorMode } from '../gen/purser/domain/v1/common_pb'
import {
  createExternalID,
  getExternalIDByValue,
} from '../gen/purser/domain/v1/external_id-ExternalIDService_connectquery'
import { createPerson } from '../gen/purser/domain/v1/person-PersonService_connectquery'
import { Gender } from '../gen/purser/domain/v1/person_pb'

const MBZ_SOURCE = 'mbz'

function isNotFound(err: unknown): boolean {
  return ConnectError.from(err).code === Code.NotFound
}

export interface MusicBrainzPersonCandidate {
  mbid: string
  name: string
  sortName?: string
  // nationality/birthDate/deathDate are only meaningful for a candidate
  // already known to be a MusicBrainz "Person" (#723's own search picker
  // filters to that before this is ever called) — the band-member/
  // solo-artist call sites in useAddArtist have none of this data
  // (MusicBrainzArtistMember carries no country/life-span), so all three
  // stay optional and are simply omitted from CreatePerson when unset.
  nationality?: string
  birthDate?: Timestamp
  deathDate?: Timestamp
}

// useGetOrCreatePersonFromMusicBrainz runs ADR 0026's 3-step get-or-create
// pattern for anyone identified by a MusicBrainz mbid — a band member, a
// solo artist's own Person record (useAddArtist), or #723's standalone
// "Search MusicBrainz" entry point on the Add Person dialog:
//
//  1. GetExternalIDByValue(PERSON, mbz, mbid) — hit → reuse the existing
//     Person, done.
//  2. Miss → speculative CreatePerson from the candidate.
//  3. CreateExternalID linking it. A returned entity_id different from the
//     Person just created means this caller lost the race to another
//     concurrent create — the loser is simply discarded (unlike
//     LibraryEntry's get-or-create, nothing else yet references the
//     speculative Person's id, so there's nothing to delete).
//
// Gender.UNKNOWN is the only honest default — MusicBrainz's artist search/
// relations data carries no gender, same "leave it unguessed" rule
// PersonDialog's own create defaults already use.
export function useGetOrCreatePersonFromMusicBrainz() {
  const getExternalIDByValueMutation = useMutation(getExternalIDByValue)
  const createPersonMutation = useMutation(createPerson)
  const createExternalIDMutation = useMutation(createExternalID)

  const getOrCreatePerson = useCallback(
    async (candidate: MusicBrainzPersonCandidate): Promise<string> => {
      try {
        const hit = await getExternalIDByValueMutation.mutateAsync({
          entityType: EntityType.PERSON,
          source: MBZ_SOURCE,
          value: candidate.mbid,
        })
        const entityId = hit.externalId?.entityId
        if (entityId) return entityId
      } catch (err) {
        if (!isNotFound(err)) throw err
      }

      const created = await createPersonMutation.mutateAsync({
        person: {
          name: candidate.name,
          sortName: candidate.sortName ?? candidate.name,
          gender: Gender.UNKNOWN,
          nationality: candidate.nationality ?? '',
          birthDate: candidate.birthDate,
          deathDate: candidate.deathDate,
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
        externalId: { entityType: EntityType.PERSON, entityId: createdPerson.id, source: MBZ_SOURCE, value: candidate.mbid },
      })
      const winnerId = linked.externalId?.entityId
      return winnerId && winnerId !== createdPerson.id ? winnerId : createdPerson.id
    },
    [getExternalIDByValueMutation, createPersonMutation, createExternalIDMutation],
  )

  return { getOrCreatePerson }
}
