import { useState } from 'react'
import { Music2, Plus, Search, X, ChevronRight, Loader2 } from 'lucide-react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useLibraryEntries } from '../../api/library'
import { searchStudios, importStudio } from '../../api/metadata'
import type { ImportStudioRequest, AlbumFilterToken } from '../../api/metadata'
import type { ExternalStudio } from '../../types'
import { PageHeader } from '../../components/layout/PageHeader'
import { EntryCard } from '../../components/media/EntryCard'
import { Pagination } from '../../components/ui/Pagination'
import { SkeletonGrid } from '../../components/ui/Skeleton'
import { EmptyState } from '../../components/ui/EmptyState'

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

// ── Add Artist dialog ─────────────────────────────────────────────────────────

type DialogState =
  | { step: 'search'; query: string; loading: boolean; error?: string }
  | { step: 'results'; query: string; results: ExternalStudio[] }
  | { step: 'edit'; candidate: ExternalStudio; form: ImportStudioRequest }
  | { step: 'saving' }

function AddArtistDialog({ onClose, onImported }: { onClose: () => void; onImported: () => void }) {
  const queryClient = useQueryClient()
  const [state, setState] = useState<DialogState>({ step: 'search', query: '', loading: false })

  const importMutation = useMutation({
    mutationFn: importStudio,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['library-entries'] })
      onImported()
    },
  })

  async function handleSearch(q: string) {
    if (!q.trim()) return
    setState({ step: 'search', query: q, loading: true })
    try {
      const data = await searchStudios(q, 'music')
      setState({ step: 'results', query: q, results: data.results })
    } catch (e) {
      setState({ step: 'search', query: q, loading: false, error: (e as Error).message })
    }
  }

  function handlePick(candidate: ExternalStudio) {
    setState({ step: 'edit', candidate, form: artistImportRequest(candidate) })
  }

  function handleSave() {
    if (state.step !== 'edit') return
    importMutation.mutate(state.form)
    setState({ step: 'saving' })
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4" style={{ background: 'rgba(0,0,0,0.7)' }}>
      <div className="w-full max-w-lg rounded-xl border border-white/10 shadow-2xl flex flex-col" style={{ background: '#0f0f17', maxHeight: '80vh' }}>

        {/* Header */}
        <div className="flex items-center justify-between px-5 py-4 border-b border-white/8">
          <h2 className="text-sm font-semibold text-white">Add Artist</h2>
          <button onClick={onClose} className="text-white/40 hover:text-white/70 transition-colors">
            <X size={16} />
          </button>
        </div>

        {/* Body */}
        <div className="flex-1 overflow-y-auto px-5 py-4 min-h-0">

          {/* Search step */}
          {(state.step === 'search' || state.step === 'results') && (
            <SearchStep
              initialQuery={state.step === 'results' ? state.query : (state as { query: string }).query}
              loading={state.step === 'search' && (state as { loading: boolean }).loading}
              error={state.step === 'search' ? (state as { error?: string }).error : undefined}
              onSearch={handleSearch}
            />
          )}

          {/* Results list */}
          {state.step === 'results' && (
            <div className="mt-4 space-y-1">
              {state.results.length === 0 ? (
                <p className="text-sm text-white/40 text-center py-6">No results found</p>
              ) : (
                state.results.map((s, i) => (
                  <ArtistResult key={`${s.source}-${s.externalId}-${i}`} artist={s} onPick={handlePick} />
                ))
              )}
            </div>
          )}

          {/* Edit step */}
          {state.step === 'edit' && (
            <EditForm
              form={state.form}
              onChange={form => setState({ step: 'edit', candidate: state.candidate, form })}
            />
          )}

          {/* Saving */}
          {state.step === 'saving' && (
            <div className="flex items-center justify-center gap-3 py-12 text-white/50">
              <Loader2 size={18} className="animate-spin" />
              <span className="text-sm">Adding artist…</span>
            </div>
          )}
        </div>

        {/* Footer */}
        {state.step === 'edit' && (
          <div className="px-5 py-4 border-t border-white/8 flex items-center justify-between gap-3">
            <button
              onClick={() => setState({ step: 'results', query: '', results: [] })}
              className="text-sm text-white/40 hover:text-white/70 transition-colors"
            >
              ← Back
            </button>
            <button
              onClick={handleSave}
              className="px-4 py-1.5 rounded-lg text-sm font-medium text-white transition-colors"
              style={{ background: ACCENT }}
            >
              Add Artist
            </button>
          </div>
        )}
      </div>
    </div>
  )
}

function SearchStep({ initialQuery, loading, error, onSearch }: {
  initialQuery: string
  loading: boolean
  error?: string
  onSearch: (q: string) => void
}) {
  const [q, setQ] = useState(initialQuery)
  return (
    <div>
      <p className="text-xs text-white/40 mb-3">Search MusicBrainz for an artist to add to your library.</p>
      <form onSubmit={e => { e.preventDefault(); onSearch(q) }} className="flex gap-2">
        <input
          autoFocus
          value={q}
          onChange={e => setQ(e.target.value)}
          placeholder="e.g. Radiohead"
          className="flex-1 bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-sm text-white placeholder-white/25 outline-none focus:border-white/20"
        />
        <button
          type="submit"
          disabled={loading || !q.trim()}
          className="flex items-center gap-1.5 px-3 py-2 rounded-lg text-sm font-medium text-white disabled:opacity-40 transition-colors"
          style={{ background: ACCENT }}
        >
          {loading ? <Loader2 size={14} className="animate-spin" /> : <Search size={14} />}
          Search
        </button>
      </form>
      {error && <p className="mt-2 text-xs text-red-400">{error}</p>}
    </div>
  )
}

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

function EditForm({ form, onChange }: { form: ImportStudioRequest; onChange: (f: ImportStudioRequest) => void }) {
  return (
    <div className="space-y-4">
      {/* Artist image preview */}
      {form.imageUrl && (
        <div className="flex items-center gap-3">
          <img src={form.imageUrl} alt={form.name} className="w-12 h-12 object-cover rounded-full bg-white/5" />
          <p className="text-xs text-white/35">Image from {form.source}</p>
        </div>
      )}

      {/* Name */}
      <div>
        <label className="block text-xs text-white/40 mb-1">Name</label>
        <input
          value={form.name}
          onChange={e => onChange({ ...form, name: e.target.value })}
          className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-sm text-white outline-none focus:border-white/20"
        />
      </div>

      {/* Overview */}
      <div>
        <label className="block text-xs text-white/40 mb-1">Overview</label>
        <textarea
          rows={3}
          value={form.overview ?? ''}
          onChange={e => onChange({ ...form, overview: e.target.value })}
          className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-sm text-white outline-none focus:border-white/20 resize-none"
        />
      </div>

      {/* Album types */}
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

      {/* Monitor mode */}
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

      {/* Monitored */}
      <label className="flex items-center gap-3 cursor-pointer">
        <div
          onClick={() => onChange({ ...form, monitored: !form.monitored })}
          className={`w-9 h-5 rounded-full transition-colors relative ${form.monitored ? '' : 'bg-white/10'}`}
          style={form.monitored ? { background: ACCENT } : {}}
        >
          <div className={`absolute top-0.5 w-4 h-4 rounded-full bg-white shadow transition-transform ${form.monitored ? 'translate-x-4' : 'translate-x-0.5'}`} />
        </div>
        <span className="text-sm text-white/70">Monitored</span>
      </label>

      {/* Source badge */}
      <p className="text-xs text-white/25">
        Source: <span className="uppercase tracking-wide">{form.source}</span> · ID: {form.externalId}
      </p>
    </div>
  )
}

// ── Page ──────────────────────────────────────────────────────────────────────

export function MusicPage() {
  const [search, setSearch] = useState('')
  const [offset, setOffset] = useState(0)
  const [showAdd, setShowAdd] = useState(false)

  const resetPage = (v: string) => { setSearch(v); setOffset(0) }

  const { data, isLoading, refetch } = useLibraryEntries({
    contentType: 'music',
    kind: 'artist',
    search: search || undefined,
    limit: LIMIT,
    offset,
  })

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

      {showAdd && (
        <AddArtistDialog
          onClose={() => setShowAdd(false)}
          onImported={() => {
            setShowAdd(false)
            void refetch()
          }}
        />
      )}
    </div>
  )
}
