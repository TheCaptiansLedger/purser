import { useState } from 'react'
import { Music2, Plus, ChevronRight } from 'lucide-react'
import { useQueryClient } from '@tanstack/react-query'
import { useLibraryEntries } from '../../api/library'
import { searchStudios, importStudio } from '../../api/metadata'
import type { ImportStudioRequest, AlbumFilterToken } from '../../api/metadata'
import type { ExternalStudio } from '../../types'
import { PageHeader } from '../../components/layout/PageHeader'
import { EntryCard } from '../../components/media/EntryCard'
import { Pagination } from '../../components/ui/Pagination'
import { SkeletonGrid } from '../../components/ui/Skeleton'
import { EmptyState } from '../../components/ui/EmptyState'
import { ImportDialog } from '../../components/ImportDialog'

const ACCENT = '#10b981'
const LIMIT = 48

export function artistImportRequest(candidate: ExternalStudio): ImportStudioRequest {
  return {
    source: candidate.source,
    externalId: candidate.externalId,
    name: candidate.name,
    overview: candidate.overview ?? '',
    contentType: 'music',
    kind: 'artist',
    monitored: true,
    monitorMode: 'all',
    imageUrl: candidate.imageUrl,
    albumFilter: ['studio', 'live'],
  }
}

const ALBUM_FILTER_OPTIONS: { value: Exclude<AlbumFilterToken, 'all'>; label: string }[] = [
  { value: 'studio', label: 'Studio albums' },
  { value: 'live', label: 'Live albums' },
  { value: 'compilation', label: 'Compilations' },
  { value: 'ep', label: 'EPs' },
  { value: 'single', label: 'Singles' },
]

function ArtistResult({ artist, onPick }: { artist: ExternalStudio; onPick: (s: ExternalStudio) => void }) {
  return (
    <button
      onClick={() => onPick(artist)}
      className="w-full flex items-center gap-3 px-3 py-2.5 rounded-lg hover:bg-white/5 transition-colors text-left group"
    >
      {artist.imageUrl ? (
        <img src={artist.imageUrl} alt={artist.name} className="w-10 h-10 object-cover rounded-full shrink-0 bg-white/5" />
      ) : (
        <div className="w-10 h-10 rounded-full shrink-0 bg-white/5 flex items-center justify-center">
          <Music2 size={14} className="text-white/20" />
        </div>
      )}
      <div className="flex-1 min-w-0">
        <p className="text-sm font-medium text-white truncate">{artist.name}</p>
      </div>
      <span className="text-xs text-white/25 shrink-0 uppercase tracking-wide">{artist.source}</span>
      <ChevronRight size={14} className="text-white/20 group-hover:text-white/50 transition-colors shrink-0" />
    </button>
  )
}

function ArtistEditForm({ form, onChange }: { form: ImportStudioRequest; onChange: (f: ImportStudioRequest) => void }) {
  return (
    <div className="space-y-4">
      {form.imageUrl && (
        <div className="flex items-center gap-3">
          <img src={form.imageUrl} alt={form.name} className="w-12 h-12 object-cover rounded-full bg-white/5" />
          <p className="text-xs text-white/35">Image from {form.source}</p>
        </div>
      )}

      <div>
        <label className="block text-xs text-white/40 mb-1">Name</label>
        <input
          value={form.name}
          onChange={e => onChange({ ...form, name: e.target.value })}
          className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-sm text-white outline-none focus:border-white/20"
        />
      </div>

      <div>
        <label className="block text-xs text-white/40 mb-1">Overview</label>
        <textarea
          rows={3}
          value={form.overview ?? ''}
          onChange={e => onChange({ ...form, overview: e.target.value })}
          className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-sm text-white outline-none focus:border-white/20 resize-none"
        />
      </div>

      <div>
        <label className="block text-xs text-white/40 mb-2">Album types to import</label>
        <div className="flex flex-wrap gap-2">
          {ALBUM_FILTER_OPTIONS.map(opt => {
            const isAll = form.albumFilter?.includes('all')
            const active = isAll || (form.albumFilter ?? []).includes(opt.value)
            return (
              <button
                key={opt.value}
                type="button"
                onClick={() => {
                  const current = (form.albumFilter ?? []).filter(v => v !== 'all')
                  const next = current.includes(opt.value)
                    ? current.filter(v => v !== opt.value)
                    : [...current, opt.value]
                  onChange({ ...form, albumFilter: next.length > 0 ? next : ['studio'] })
                }}
                className="px-3 py-1 rounded-full text-xs font-medium border transition-colors"
                style={active
                  ? { background: ACCENT + '22', borderColor: ACCENT, color: ACCENT }
                  : { background: 'transparent', borderColor: 'rgba(255,255,255,0.12)', color: 'rgba(255,255,255,0.4)' }
                }
              >
                {opt.label}
              </button>
            )
          })}
          <button
            type="button"
            onClick={() => onChange({ ...form, albumFilter: ['all'] })}
            className="px-3 py-1 rounded-full text-xs font-medium border transition-colors"
            style={(form.albumFilter ?? []).includes('all')
              ? { background: ACCENT + '22', borderColor: ACCENT, color: ACCENT }
              : { background: 'transparent', borderColor: 'rgba(255,255,255,0.12)', color: 'rgba(255,255,255,0.4)' }
            }
          >
            All
          </button>
        </div>
      </div>

      <div>
        <label className="block text-xs text-white/40 mb-1">Import mode</label>
        <select
          value={form.monitorMode}
          onChange={e => onChange({ ...form, monitorMode: e.target.value as ImportStudioRequest['monitorMode'] })}
          className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-sm text-white outline-none focus:border-white/20"
        >
          <option value="all">All — mark every album as wanted</option>
          <option value="future">Future — only albums released after today are wanted</option>
          <option value="none">None — import albums without marking any as wanted</option>
          <option value="latest">Latest only — only the most recent album is wanted</option>
        </select>
      </div>

      <label className="flex items-center gap-3 cursor-pointer">
        <button
          type="button"
          role="switch"
          aria-checked={form.monitored}
          onClick={() => onChange({ ...form, monitored: !form.monitored })}
          className={`w-9 h-5 rounded-full transition-colors relative ${form.monitored ? '' : 'bg-white/10'}`}
          style={form.monitored ? { background: ACCENT } : {}}
        >
          <div className={`absolute top-0.5 w-4 h-4 rounded-full bg-white shadow transition-transform ${form.monitored ? 'translate-x-4' : 'translate-x-0.5'}`} />
        </button>
        <span className="text-sm text-white/70">Monitored</span>
      </label>

      <p className="text-xs text-white/25">
        Source: <span className="uppercase tracking-wide">{form.source}</span> · ID: {form.externalId}
      </p>
    </div>
  )
}

// ── Page ──────────────────────────────────────────────────────────────────────

export function MusicPage() {
  const queryClient = useQueryClient()
  const [search, setSearch] = useState('')
  const [offset, setOffset] = useState(0)
  const [showAdd, setShowAdd] = useState(false)

  const resetPage = (v: string) => { setSearch(v); setOffset(0) }

  const { data, isLoading } = useLibraryEntries({
    contentType: 'music',
    kind: 'artist',
    search: search || undefined,
    limit: LIMIT,
    offset,
  })

  async function handleImport(form: ImportStudioRequest) {
    await importStudio(form)
    queryClient.invalidateQueries({ queryKey: ['library-entries'] })
  }

  return (
    <div>
      <PageHeader title="Music" accent={ACCENT} search={search} onSearch={resetPage} total={data?.total}>
        <button
          onClick={() => setShowAdd(true)}
          className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium text-white transition-colors shrink-0"
          style={{ background: ACCENT }}
        >
          <Plus size={14} />
          Add Artist
        </button>
      </PageHeader>
      <div className="px-8 py-6">
        {isLoading ? (
          <SkeletonGrid count={24} aspect="1/1" />
        ) : !data?.data.length ? (
          <EmptyState icon={Music2} title="No artists yet" description="Add music to your library to see artists here." accent={ACCENT} />
        ) : (
          <>
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4">
              {data.data.map(entry => (
                <EntryCard key={entry.id} entry={entry} href={`/music/${entry.id}`} aspect="1/1" accent={ACCENT} />
              ))}
            </div>
            <Pagination total={data.total} limit={LIMIT} offset={offset} onChange={setOffset} accent={ACCENT} />
          </>
        )}
      </div>

      <ImportDialog<ExternalStudio, ImportStudioRequest>
        open={showAdd}
        onClose={() => setShowAdd(false)}
        title="Add Artist"
        accent={ACCENT}
        searchHint="Search MusicBrainz for an artist to add to your library."
        searchPlaceholder="e.g. Radiohead"
        savingLabel="Adding artist…"
        keyOf={s => `${s.source}-${s.externalId}`}
        onSearch={q => searchStudios(q, 'music').then(r => r.results)}
        buildForm={artistImportRequest}
        renderResult={(item, onSelect) => <ArtistResult artist={item} onPick={onSelect} />}
        renderEditForm={(form, onChange) => <ArtistEditForm form={form} onChange={onChange} />}
        onImport={handleImport}
      />
    </div>
  )
}
