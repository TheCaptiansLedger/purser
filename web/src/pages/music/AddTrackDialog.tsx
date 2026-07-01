import { useState, useEffect } from 'react'
import { X, Loader2 } from 'lucide-react'
import { useQueryClient } from '@tanstack/react-query'
import { searchTracks, importTrack } from '../../api/metadata'
import { RuntimeInput } from '../../components/edit/fields/RuntimeInput'
import { Toggle } from '../../components/edit/fields/Toggle'
import { fmtRuntime } from '../../components/ui/Runtime'
import type { ContentType, ExternalID, ExternalTrack } from '../../types'

interface TrackForm {
  title: string
  sequence: string
  runtimeSeconds: number
  monitored: boolean
}

function blankForm(): TrackForm {
  return { title: '', sequence: '', runtimeSeconds: 0, monitored: true }
}

interface AddTrackDialogProps {
  open: boolean
  onClose: () => void
  albumId: string
  albumExternalIds: ExternalID[]
  libraryEntryId: string
  contentType: ContentType
  accent: string
}

export function AddTrackDialog({
  open,
  onClose,
  albumId,
  albumExternalIds,
  libraryEntryId,
  contentType,
  accent,
}: AddTrackDialogProps) {
  const queryClient = useQueryClient()
  const [tracks, setTracks] = useState<ExternalTrack[]>([])
  const [loading, setLoading] = useState(false)
  const [filter, setFilter] = useState('')
  const [form, setForm] = useState<TrackForm>(blankForm)
  const [selectedTrack, setSelectedTrack] = useState<ExternalTrack | undefined>()
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | undefined>()

  const mbid = albumExternalIds.find(e => e.source === 'mbz')?.value

  useEffect(() => {
    if (!open) return
    setForm(blankForm())
    setSelectedTrack(undefined)
    setFilter('')
    setError(undefined)
    if (!mbid) {
      setTracks([])
      return
    }
    setLoading(true)
    searchTracks('mbz', contentType, mbid)
      .then(res => setTracks(res.results))
      .catch(() => setTracks([]))
      .finally(() => setLoading(false))
  }, [open, mbid, contentType])

  if (!open) return null

  const visibleTracks = filter.trim()
    ? tracks.filter(t => t.title.toLowerCase().includes(filter.toLowerCase()))
    : tracks

  function handlePick(track: ExternalTrack) {
    setSelectedTrack(track)
    setForm(f => ({
      ...f,
      title: track.title,
      sequence: track.sequence ?? '',
      runtimeSeconds: track.runtimeSeconds ?? 0,
    }))
  }

  async function handleSave() {
    if (!form.title.trim()) return
    setSaving(true)
    setError(undefined)
    try {
      await importTrack({
        source: selectedTrack?.source,
        externalId: selectedTrack?.externalId,
        groupId: albumId,
        libraryEntryId,
        contentType,
        title: form.title,
        sequence: form.sequence || undefined,
        runtimeSeconds: form.runtimeSeconds || undefined,
        monitored: form.monitored,
      })
      await queryClient.invalidateQueries({ queryKey: ['items'] })
      onClose()
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4" style={{ background: 'rgba(0,0,0,0.7)' }}>
      <div className="w-full max-w-lg rounded-xl border border-white/10 shadow-2xl flex flex-col" style={{ background: '#0f0f17', maxHeight: '85vh' }}>

        <div className="flex items-center justify-between px-5 py-4 border-b border-white/8">
          <h2 className="text-sm font-semibold text-white">Add Track</h2>
          <button onClick={onClose} className="text-white/40 hover:text-white/70 transition-colors">
            <X size={16} />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto px-5 py-4 min-h-0 space-y-4">

          {mbid && (
            <div>
              {loading ? (
                <div className="flex items-center gap-2 text-xs text-white/40 py-2">
                  <Loader2 size={12} className="animate-spin" />
                  Loading tracks from MusicBrainz…
                </div>
              ) : tracks.length > 0 ? (
                <>
                  <input
                    value={filter}
                    onChange={e => setFilter(e.target.value)}
                    placeholder="Filter tracks…"
                    className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-sm text-white placeholder-white/25 outline-none focus:border-white/20 mb-2"
                  />
                  <div className="border border-white/8 rounded-lg overflow-hidden" style={{ maxHeight: '13rem', overflowY: 'auto' }}>
                    {visibleTracks.map(track => {
                      const active = selectedTrack?.externalId === track.externalId
                      return (
                        <button
                          key={track.externalId}
                          onClick={() => handlePick(track)}
                          className="w-full flex items-center gap-3 px-3 py-2 text-left text-sm transition-colors hover:bg-white/5"
                          style={active ? { background: accent + '1a' } : {}}
                        >
                          <span className="w-6 text-right text-xs text-white/30 shrink-0">{track.sequence ?? '—'}</span>
                          <span className="flex-1 truncate" style={{ color: active ? accent : 'rgba(255,255,255,0.75)' }}>
                            {track.title}
                          </span>
                          {(track.runtimeSeconds ?? 0) > 0 && (
                            <span className="text-xs text-white/30 shrink-0">{fmtRuntime(track.runtimeSeconds!)}</span>
                          )}
                        </button>
                      )
                    })}
                    {visibleTracks.length === 0 && (
                      <p className="text-xs text-white/40 text-center py-4">No tracks match</p>
                    )}
                  </div>
                </>
              ) : (
                <p className="text-xs text-white/30 py-1">No MusicBrainz tracks found — enter manually below.</p>
              )}
            </div>
          )}

          {!mbid && (
            <p className="text-xs text-white/30">
              No MusicBrainz ID on this album — track will be entered manually.
            </p>
          )}

          {selectedTrack && (
            <p className="text-xs" style={{ color: accent + 'aa' }}>
              Seeded from MusicBrainz · {selectedTrack.externalId}
            </p>
          )}

          <div>
            <label className="block text-xs text-white/40 mb-1">
              Title <span className="text-red-400">*</span>
            </label>
            <input
              autoFocus={!mbid}
              value={form.title}
              onChange={e => setForm(f => ({ ...f, title: e.target.value }))}
              onKeyDown={e => { if (e.key === 'Enter' && form.title.trim()) void handleSave() }}
              placeholder="Track title"
              className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-sm text-white placeholder-white/25 outline-none focus:border-white/20"
            />
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-xs text-white/40 mb-1">Track #</label>
              <input
                value={form.sequence}
                onChange={e => setForm(f => ({ ...f, sequence: e.target.value }))}
                placeholder="e.g. 3 or A1"
                className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-sm text-white placeholder-white/25 outline-none focus:border-white/20"
              />
            </div>
            <div>
              <label className="block text-xs text-white/40 mb-1">Runtime</label>
              <RuntimeInput
                value={form.runtimeSeconds}
                onChange={v => setForm(f => ({ ...f, runtimeSeconds: v }))}
              />
            </div>
          </div>

          <label className="flex items-center gap-3 cursor-pointer">
            <Toggle value={form.monitored} onChange={v => setForm(f => ({ ...f, monitored: v }))} />
            <span className="text-sm text-white/70">Monitored</span>
          </label>

          {error && <p className="text-xs text-red-400">{error}</p>}

        </div>

        <div className="px-5 py-4 border-t border-white/8 flex justify-end gap-3">
          <button
            onClick={onClose}
            className="px-4 py-1.5 rounded-lg text-sm text-white/50 hover:text-white/80 transition-colors"
          >
            Cancel
          </button>
          <button
            onClick={() => { void handleSave() }}
            disabled={!form.title.trim() || saving}
            className="px-4 py-1.5 rounded-lg text-sm font-medium text-white transition-colors disabled:opacity-40"
            style={{ background: accent }}
          >
            {saving ? 'Adding…' : 'Add Track'}
          </button>
        </div>

      </div>
    </div>
  )
}
