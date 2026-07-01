import { useState, useEffect } from 'react'
import { X, Database } from 'lucide-react'
import { useQueryClient } from '@tanstack/react-query'
import { importAlbum } from '../../api/metadata'
import { createGroup } from '../../api/groups'
import { Toggle } from '../../components/edit/fields/Toggle'
import { DiscographyBrowser } from './DiscographyBrowser'
import type { ImportAlbumRequest } from '../../api/metadata'
import type { CreateGroupRequest } from '../../api/groups'
import type { ExternalGroup, MonitorMode } from '../../types'

// ── Release type ───────────────────────────────────────────────────────────────

export type ReleaseType = 'studio' | 'live' | 'ep_single' | 'compilation' | 'other'

const RELEASE_TYPE_OPTIONS: { value: ReleaseType; label: string }[] = [
  { value: 'studio',      label: 'Album'       },
  { value: 'live',        label: 'Live'        },
  { value: 'ep_single',   label: 'EP / Single' },
  { value: 'compilation', label: 'Compilation' },
  { value: 'other',       label: 'Other'       },
]

export function releaseTypeFromExternal(primaryType?: string, secondaryTypes?: string[]): ReleaseType {
  if (primaryType === 'EP' || primaryType === 'Single') return 'ep_single'
  if (primaryType !== 'Album') return 'other'
  if (secondaryTypes?.includes('Live')) return 'live'
  if (secondaryTypes?.includes('Compilation')) return 'compilation'
  return 'studio'
}

export function releaseTypeToMetadata(releaseType: ReleaseType): Record<string, unknown> {
  switch (releaseType) {
    case 'studio':      return { primary_type: 'Album', secondary_types: [] }
    case 'live':        return { primary_type: 'Album', secondary_types: ['Live'] }
    case 'compilation': return { primary_type: 'Album', secondary_types: ['Compilation'] }
    case 'ep_single':   return { primary_type: 'EP',    secondary_types: [] }
    default:            return {}
  }
}

// ── Form ───────────────────────────────────────────────────────────────────────

export interface AlbumForm {
  title: string
  year: string
  overview: string
  releaseType: ReleaseType
  monitored: boolean
  monitorMode: MonitorMode
}

export function blankAlbumForm(): AlbumForm {
  return { title: '', year: '', overview: '', releaseType: 'studio', monitored: true, monitorMode: 'all' }
}

type SelectedAlbum = { source: string; externalId: string }

export function toImportRequest(
  form: AlbumForm,
  libraryEntryId: string,
  selected: SelectedAlbum,
): ImportAlbumRequest {
  const meta = releaseTypeToMetadata(form.releaseType)
  return {
    source: selected.source,
    externalId: selected.externalId,
    libraryEntryId,
    title: form.title,
    year: form.year ? Number(form.year) : undefined,
    monitored: form.monitored,
    monitorMode: form.monitorMode,
    primaryType: meta.primary_type as string | undefined,
    secondaryTypes: meta.secondary_types as string[] | undefined,
  }
}

export function toCreateRequest(form: AlbumForm, libraryEntryId: string): CreateGroupRequest {
  return {
    libraryEntryId,
    title: form.title,
    year: form.year ? Number(form.year) : undefined,
    overview: form.overview || undefined,
    monitored: form.monitored,
    monitorMode: form.monitorMode,
    metadata: releaseTypeToMetadata(form.releaseType),
  }
}

// ── Dialog ────────────────────────────────────────────────────────────────────

interface AddAlbumDialogProps {
  open: boolean
  onClose: () => void
  artistLibraryEntryId: string
  artistMbid?: string
  importedMbids: Set<string>
  accent: string
}

export function AddAlbumDialog({
  open,
  onClose,
  artistLibraryEntryId,
  artistMbid,
  importedMbids,
  accent,
}: AddAlbumDialogProps) {
  const queryClient = useQueryClient()
  const [form, setForm] = useState<AlbumForm>(blankAlbumForm)
  const [selectedAlbum, setSelectedAlbum] = useState<SelectedAlbum | undefined>()
  const [browseOpen, setBrowseOpen] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | undefined>()

  useEffect(() => {
    if (open) {
      setForm(blankAlbumForm())
      setSelectedAlbum(undefined)
      setError(undefined)
    }
  }, [open])

  if (!open) return null

  function handleClose() {
    setForm(blankAlbumForm())
    setSelectedAlbum(undefined)
    setError(undefined)
    onClose()
  }

  function handlePick(album: ExternalGroup) {
    setSelectedAlbum({ source: album.source, externalId: album.externalId })
    setForm(f => ({
      ...f,
      title: album.title,
      year: album.year ? String(album.year) : '',
      releaseType: releaseTypeFromExternal(album.primaryType, album.secondaryTypes),
    }))
  }

  async function handleSave() {
    if (!form.title.trim()) return
    setSaving(true)
    setError(undefined)
    try {
      if (selectedAlbum) {
        await importAlbum(toImportRequest(form, artistLibraryEntryId, selectedAlbum))
      } else {
        await createGroup(toCreateRequest(form, artistLibraryEntryId))
      }
      await queryClient.invalidateQueries({ queryKey: ['groups', { libraryEntryId: artistLibraryEntryId }] })
      setForm(blankAlbumForm())
      setSelectedAlbum(undefined)
      onClose()
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setSaving(false)
    }
  }

  return (
    <>
      <div className="fixed inset-0 z-50 flex items-center justify-center p-4" style={{ background: 'rgba(0,0,0,0.7)' }}>
        <div className="w-full max-w-lg xl:max-w-xl 2xl:max-w-2xl rounded-xl border border-white/10 shadow-2xl flex flex-col" style={{ background: '#0f0f17', maxHeight: '85vh' }}>

          <div className="flex items-center justify-between px-5 py-4 border-b border-white/8">
            <h2 className="text-sm font-semibold text-white">Add Album</h2>
            <button onClick={handleClose} className="text-white/40 hover:text-white/70 transition-colors">
              <X size={16} />
            </button>
          </div>

          <div className="flex-1 overflow-y-auto px-5 py-4 min-h-0 space-y-4">

            <button
              type="button"
              onClick={() => setBrowseOpen(true)}
              disabled={!artistMbid}
              title={!artistMbid ? 'Artist has no MusicBrainz ID — add one via Edit to enable discography browsing' : undefined}
              className="flex items-center gap-1.5 rounded-lg border border-white/10 px-3 py-2 text-sm text-white/50 transition-colors hover:border-white/20 hover:text-white/80 disabled:opacity-40 disabled:cursor-not-allowed"
            >
              <Database size={13} />
              Browse Discography
            </button>

            {selectedAlbum && (
              <p className="text-xs" style={{ color: accent + 'aa' }}>
                Seeded from MusicBrainz · {selectedAlbum.externalId}
              </p>
            )}

            <div>
              <label className="block text-xs text-white/40 mb-1">
                Title <span className="text-red-400">*</span>
              </label>
              <input
                autoFocus
                value={form.title}
                onChange={e => setForm(f => ({ ...f, title: e.target.value }))}
                placeholder="e.g. OK Computer"
                className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-sm text-white placeholder-white/25 outline-none focus:border-white/20"
              />
            </div>

            <div>
              <label className="block text-xs text-white/40 mb-1">Year</label>
              <input
                type="number"
                value={form.year}
                onChange={e => setForm(f => ({ ...f, year: e.target.value }))}
                placeholder="e.g. 1997"
                min={1900}
                max={2100}
                className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-sm text-white placeholder-white/25 outline-none focus:border-white/20"
              />
            </div>

            <div>
              <label className="block text-xs text-white/40 mb-2">Release type</label>
              <div className="flex flex-wrap gap-2">
                {RELEASE_TYPE_OPTIONS.map(opt => {
                  const active = form.releaseType === opt.value
                  return (
                    <button
                      key={opt.value}
                      type="button"
                      onClick={() => setForm(f => ({ ...f, releaseType: opt.value }))}
                      className="px-3 py-1 rounded-full text-xs font-medium border transition-colors"
                      style={active
                        ? { background: accent + '22', borderColor: accent, color: accent }
                        : { background: 'transparent', borderColor: 'rgba(255,255,255,0.12)', color: 'rgba(255,255,255,0.4)' }
                      }
                    >
                      {opt.label}
                    </button>
                  )
                })}
              </div>
            </div>

            <div>
              <label className="block text-xs text-white/40 mb-1">Overview</label>
              <textarea
                rows={3}
                value={form.overview}
                onChange={e => setForm(f => ({ ...f, overview: e.target.value }))}
                className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-sm text-white outline-none focus:border-white/20 resize-none"
              />
            </div>

            <div>
              <label className="block text-xs text-white/40 mb-1">Monitor mode</label>
              <select
                value={form.monitorMode}
                onChange={e => setForm(f => ({ ...f, monitorMode: e.target.value as MonitorMode }))}
                className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-sm text-white outline-none focus:border-white/20"
              >
                <option value="all">All — mark every track as wanted</option>
                <option value="future">Future — only tracks released after today are wanted</option>
                <option value="none">None — import without marking as wanted</option>
                <option value="latest">Latest only — only the most recent track is wanted</option>
              </select>
            </div>

            <label className="flex items-center gap-3 cursor-pointer">
              <Toggle value={form.monitored} onChange={v => setForm(f => ({ ...f, monitored: v }))} />
              <span className="text-sm text-white/70">Monitored</span>
            </label>

            {error && <p className="text-xs text-red-400">{error}</p>}

          </div>

          <div className="px-5 py-4 border-t border-white/8 flex justify-end">
            <button
              onClick={() => { void handleSave() }}
              disabled={!form.title.trim() || saving}
              className="px-4 py-1.5 rounded-lg text-sm font-medium text-white transition-colors disabled:opacity-40"
              style={{ background: accent }}
            >
              {saving ? 'Adding…' : 'Add Album'}
            </button>
          </div>

        </div>
      </div>

      {artistMbid && (
        <DiscographyBrowser
          open={browseOpen}
          artistMbid={artistMbid}
          importedMbids={importedMbids}
          accent={accent}
          onClose={() => setBrowseOpen(false)}
          onPick={handlePick}
        />
      )}
    </>
  )
}
