import { useMutation, useQuery } from '@connectrpc/connect-query'
import { Play, Plus, Sparkles, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { createMusicReleaseTrack } from '../gen/purser/music/v1/release-MusicReleaseService_connectquery'
import { getRelease } from '../gen/purser/music/v1/musicbrainz_search-MusicBrainzService_connectquery'
import type { Item } from '../gen/purser/domain/v1/item_pb'
import { ItemStatus } from '../gen/purser/domain/v1/common_pb'
import { itemStatusFromProto } from '../hooks/useReleaseTracks'
import { useTrackMediaFilePresence } from '../hooks/useTrackMediaFilePresence'
import { formatRuntime } from '../lib/formatRuntime'
import { ItemStatusBadge } from './ItemStatusBadge'
import { EmptyState } from './EmptyState'
import { AddTrackDialog } from './AddTrackDialog'
import { TrackDeleteDialog } from './TrackDeleteDialog'

export interface TracklistProps {
  releaseId: string
  // mbid — the selected edition's own MBID, or "" when it has none (a
  // manually-added edition). Gates "Populate from MusicBrainz" per this
  // issue's own scope note.
  mbid: string
  tracks: Item[]
  isPending: boolean
  refetch: () => void
}

interface PopulateResult {
  added: number
  total: number
  failed: string[]
}

function discNumberOf(track: Item): number {
  const raw = track.metadata?.disc_number
  return typeof raw === 'number' ? raw : 1
}

function sequenceKeyOf(track: Item): number {
  const n = Number.parseInt(track.sequence, 10)
  return Number.isNaN(n) ? Number.MAX_SAFE_INTEGER : n
}

// sortedTracks orders tracks for display only (disc, then track number) —
// a client-side presentation sort of one release's own tracklist, not the
// server-side/candidate-list ranking this codebase otherwise avoids.
function sortedTracks(tracks: Item[]): Item[] {
  return [...tracks].sort((a, b) => discNumberOf(a) - discNumberOf(b) || sequenceKeyOf(a) - sequenceKeyOf(b))
}

// Tracklist is #676's per-edition track table: ItemStatusBadge (#659) per
// row, a Play icon rendered only when Status=imported AND
// MediaFileService.ListMediaFiles resolves a real file — visibly present,
// functionally inert (playback itself is #643, not wired here) — and
// Add track (manual + Populate from MusicBrainz) + Delete, the write path
// this tab had none of before (#721/#720).
export function Tracklist({ releaseId, mbid, tracks, isPending, refetch }: TracklistProps) {
  const [addOpen, setAddOpen] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<Item | null>(null)
  const [isPopulating, setIsPopulating] = useState(false)
  const [populateResult, setPopulateResult] = useState<PopulateResult | null>(null)

  const mediaFilePresence = useTrackMediaFilePresence(tracks)
  const createTrackMutation = useMutation(createMusicReleaseTrack)
  const releaseQuery = useQuery(getRelease, { mbid }, { enabled: false })

  // The empty-tracklist gate: absent (not just disabled) once the
  // tracklist is non-empty, absent with no mbid. A fully-failed Populate
  // (tracks.length still 0 after refetch) re-shows this button — safely
  // retryable, per this issue's own scope note. A partially-succeeded one
  // hides it for good (tracks.length > 0), leaving "Add track" to finish
  // the rest.
  const showPopulate = mbid !== '' && !isPending && tracks.length === 0 && !isPopulating

  async function handlePopulate() {
    setPopulateResult(null)
    setIsPopulating(true)

    const result = await releaseQuery.refetch()
    const media = result.data?.media ?? []
    const mbTracks = media.flatMap(medium => medium.tracks.map(track => ({ medium, track })))

    let added = 0
    const failed: string[] = []
    setPopulateResult({ added, total: mbTracks.length, failed })

    for (const { medium, track } of mbTracks) {
      try {
        await createTrackMutation.mutateAsync({
          releaseId,
          title: track.title,
          number: track.number,
          mediumNumber: medium.position,
          runtimeSeconds: Math.round(track.lengthMs / 1000),
          mbid: track.recordingMbid,
        })
        added += 1
      } catch {
        failed.push(track.title || track.number)
      }
      setPopulateResult({ added, total: mbTracks.length, failed: [...failed] })
    }

    setIsPopulating(false)
    refetch()
  }

  function handleTrackAdded() {
    setAddOpen(false)
    refetch()
  }

  function handleTrackDeleted() {
    setDeleteTarget(null)
    refetch()
  }

  const rows = sortedTracks(tracks)
  const populateDone = populateResult !== null && populateResult.added + populateResult.failed.length === populateResult.total

  return (
    <div className="mt-6">
      <div className="mb-3 flex items-center justify-between">
        <h3 className="text-title-sm text-text">Tracks</h3>
        <div className="flex items-center gap-2">
          {(showPopulate || isPopulating) && (
            <button
              type="button"
              onClick={handlePopulate}
              disabled={isPopulating}
              className="flex h-8 items-center gap-1.5 rounded-lg px-3 text-label font-medium text-text-secondary hover:bg-surface-raised hover:text-text disabled:opacity-50"
            >
              <Sparkles size={14} aria-hidden="true" />
              Populate from MusicBrainz
            </button>
          )}
          <button
            type="button"
            onClick={() => setAddOpen(true)}
            className="flex h-8 items-center gap-1.5 rounded-lg px-3 text-label font-medium text-text-secondary hover:bg-surface-raised hover:text-text"
          >
            <Plus size={14} aria-hidden="true" />
            Add track
          </button>
        </div>
      </div>

      {populateResult !== null && (
        <p className="mb-3 text-label text-text-secondary" role="status">
          {populateResult.added} of {populateResult.total} tracks added…
          {populateDone && populateResult.failed.length > 0 && (
            <span className="ml-2 text-status-failure">
              Couldn't add: {populateResult.failed.join(', ')}. Use "Add track" for these.
            </span>
          )}
        </p>
      )}

      {!isPending && rows.length === 0 && populateResult === null && (
        <EmptyState icon={Play} title="No tracks yet" description="Add tracks manually or populate them from MusicBrainz." />
      )}

      {rows.length > 0 && (
        <ul className="flex flex-col divide-y divide-border">
          {rows.map(track => {
            const status = itemStatusFromProto(track.status)
            const showPlay = track.status === ItemStatus.IMPORTED && mediaFilePresence.has(track.id)
            const runtime = formatRuntime(track.runtimeSeconds)
            return (
              <li key={track.id} className="flex items-center gap-3 py-2">
                <span className="w-10 shrink-0 text-label text-text-secondary">{track.sequence || '—'}</span>
                <span className="flex-1 truncate text-body text-text">{track.title}</span>
                {runtime && <span className="text-label text-text-secondary">{runtime}</span>}
                {status && <ItemStatusBadge status={status} />}
                {showPlay && (
                  <button
                    type="button"
                    aria-disabled="true"
                    title="Playback coming soon"
                    className="flex h-7 w-7 items-center justify-center rounded-lg text-text-secondary opacity-50 cursor-not-allowed"
                  >
                    <Play size={14} />
                  </button>
                )}
                <button
                  type="button"
                  onClick={() => setDeleteTarget(track)}
                  aria-label={`Delete ${track.title}`}
                  title="Delete track"
                  className="flex h-7 w-7 items-center justify-center rounded-lg text-text-secondary hover:bg-surface-raised hover:text-status-failure"
                >
                  <Trash2 size={14} />
                </button>
              </li>
            )
          })}
        </ul>
      )}

      {addOpen && <AddTrackDialog releaseId={releaseId} onClose={() => setAddOpen(false)} onAdded={handleTrackAdded} />}

      {deleteTarget && (
        <TrackDeleteDialog
          trackId={deleteTarget.id}
          trackTitle={deleteTarget.title}
          onClose={() => setDeleteTarget(null)}
          onDeleted={handleTrackDeleted}
        />
      )}
    </div>
  )
}
