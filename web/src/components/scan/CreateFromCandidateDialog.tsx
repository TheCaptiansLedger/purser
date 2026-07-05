import { useState } from 'react'
import { Link } from 'react-router-dom'
import { X, CheckCircle, Loader2, AlertCircle } from 'lucide-react'
import type { UnmatchedFile, MatchCandidate, ContentType } from '../../types'
import { manualMatch } from '../../api/scan'
import { importEntry, importItem, entryKindForContentType } from '../../api/metadata'
import { createItem } from '../../api/items'
import { pollForItemId } from '../../api/jobs'

interface Props {
  unmatchedId: string
  file: UnmatchedFile
  candidate: MatchCandidate
  onMatch: () => void
  onClose: () => void
}

export function CreateFromCandidateDialog({ unmatchedId, file, candidate, onMatch, onClose }: Props) {
  const [phase, setPhase]     = useState<'review' | 'running' | 'done' | 'error'>('review')
  const [errMsg, setErrMsg]   = useState<string | null>(null)
  const [createdId, setCreatedId] = useState<string | null>(null)

  const ext = candidate.external!

  const handleCreate = async () => {
    setPhase('running')
    try {
      let itemId = ''

      if (ext.external_id && ext.source) {
        // Identified file: server fetches full metadata (performers, tags, cover, etc.)
        // Prefer the release MBID surfaced by the server from the recording's
        // MBZ release list; fall back to embedded file tags for older queue entries.
        const albumExternalId = ext.group_external_id
                             ?? file.fingerprint?.embedded_tags?.musicbrainz_release_id
                             ?? file.fingerprint?.embedded_tags?.musicbrainz_album_id
        const albumTitle = file.fingerprint?.embedded_tags?.album

        const resp = await importItem({
          source: ext.source,
          externalId: ext.external_id,
          contentType: ext.content_type as ContentType,
          ...(albumExternalId ? { albumExternalId, albumTitle } : {}),
          monitored: true,
        })
        itemId = 'job_id' in resp
          ? await pollForItemId(resp.job_id)
          : resp.item.id
      } else {
        // Unidentified file: create studio manually then create a bare item.
        let libraryEntryId = ''
        if (ext.parent) {
          const result = await importEntry({
            source: ext.parent.source,
            externalId: ext.parent.external_id,
            name: ext.parent.name,
            contentType: ext.content_type as ContentType,
            kind: entryKindForContentType(ext.content_type as ContentType),
            monitored: false,
            monitorMode: 'latest',
            imageUrl: ext.parent.image_url,
            ...(ext.parent.parent_id ? {
              parentExternalId: ext.parent.parent_id,
              parentName: ext.parent.parent_name,
              parentImageUrl: ext.parent.parent_image_url,
            } : {}),
          })
          libraryEntryId = result.entry.id
        }

        if (!libraryEntryId) {
          throw new Error('Cannot create item: no library entry available. Import the parent first.')
        }

        const item = await createItem({
          contentType: ext.content_type,
          libraryEntryId,
          title: ext.title,
          monitored: true,
        })
        itemId = item.id
      }

      await manualMatch(unmatchedId, itemId)
      setCreatedId(itemId)
      setPhase('done')
    } catch (e) {
      setErrMsg((e as Error).message)
      setPhase('error')
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4" style={{ background: 'rgba(0,0,0,0.7)' }}>
      <div
        className="w-full max-w-md rounded-xl border border-white/10 shadow-2xl flex flex-col"
        style={{ background: '#0f0f17', maxHeight: '80vh' }}
      >
        <div className="flex items-center justify-between px-5 py-4 border-b border-white/8">
          <h2 className="text-sm font-semibold text-white">Import &amp; Create</h2>
          <button onClick={onClose} className="text-white/40 hover:text-white/70 transition-colors">
            <X size={16} />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto px-5 py-4 min-h-0">
          {phase === 'review' && (
            <>
              <p className="text-xs text-white/45 mb-3">
                The following item will be imported and the file matched.
              </p>
              <div className="rounded-lg border border-white/8 px-4 py-3 space-y-1">
                <p className="text-sm text-white/80">{ext.title}</p>
                {ext.parent && (
                  <p className="text-xs text-white/40">{ext.parent.name}</p>
                )}
                {ext.external_id && (
                  <p className="text-[10px] text-white/25 font-mono">{ext.source}:{ext.external_id}</p>
                )}
              </div>
            </>
          )}

          {phase === 'running' && (
            <div className="flex items-center gap-3 py-2">
              <Loader2 size={14} className="animate-spin text-indigo-400 shrink-0" />
              <p className="text-sm text-white/80">Importing…</p>
            </div>
          )}

          {phase === 'done' && (
            <div className="flex items-center gap-2 text-sm text-emerald-400">
              <CheckCircle size={16} />
              File matched successfully
            </div>
          )}

          {phase === 'error' && (
            <div className="flex items-start gap-2 text-sm text-red-400">
              <AlertCircle size={16} className="shrink-0 mt-0.5" />
              <span>{errMsg ?? 'An error occurred'}</span>
            </div>
          )}
        </div>

        <div className="px-5 py-4 border-t border-white/8 flex items-center justify-end gap-3">
          {phase === 'done' ? (
            <>
              {createdId && (
                <Link
                  to={`/items/${createdId}`}
                  className="text-sm text-indigo-400 hover:text-indigo-300 transition-colors"
                >
                  View item
                </Link>
              )}
              <button
                onClick={() => { onMatch(); onClose() }}
                className="px-4 py-1.5 rounded-lg text-sm font-medium text-white transition-colors"
                style={{ background: '#10b981' }}
              >
                Done
              </button>
            </>
          ) : phase === 'error' ? (
            <>
              <button onClick={onClose} className="text-sm text-white/40 hover:text-white/70">Cancel</button>
              <button
                onClick={() => { setPhase('review'); setErrMsg(null) }}
                className="px-4 py-1.5 rounded-lg text-sm font-medium text-white/70 border border-white/10 hover:border-white/20 transition-colors"
              >
                Retry
              </button>
            </>
          ) : phase === 'review' ? (
            <>
              <button onClick={onClose} className="text-sm text-white/40 hover:text-white/70">Cancel</button>
              <button
                onClick={() => { void handleCreate() }}
                className="px-4 py-1.5 rounded-lg text-sm font-medium text-white transition-colors"
                style={{ background: '#6366f1' }}
              >
                Import &amp; Create
              </button>
            </>
          ) : null}
        </div>
      </div>
    </div>
  )
}
