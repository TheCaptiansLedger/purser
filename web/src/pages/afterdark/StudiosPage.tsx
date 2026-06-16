import { useState } from 'react'
import { Building2, Plus, Search, X, ChevronRight, Loader2 } from 'lucide-react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useLibraryEntries } from '../../api/library'
import { searchStudios, importStudio } from '../../api/metadata'
import type { ImportStudioRequest } from '../../api/metadata'
import type { ExternalStudio } from '../../types'
import { PageHeader } from '../../components/layout/PageHeader'
import { EntryCard } from '../../components/media/EntryCard'
import { Pagination } from '../../components/ui/Pagination'
import { SkeletonGrid } from '../../components/ui/Skeleton'
import { EmptyState } from '../../components/ui/EmptyState'

const ACCENT = '#f43f5e'
const LIMIT = 48

// ── Add Studio dialog ─────────────────────────────────────────────────────────

type DialogState =
  | { step: 'search'; query: string; loading: boolean; error?: string }
  | { step: 'results'; query: string; results: ExternalStudio[] }
  | { step: 'edit'; candidate: ExternalStudio; form: ImportStudioRequest }
  | { step: 'saving' }

function AddStudioDialog({ onClose, onImported }: { onClose: () => void; onImported: () => void }) {
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
      const data = await searchStudios(q, 'adult')
      setState({ step: 'results', query: q, results: data.results })
    } catch (e) {
      setState({ step: 'search', query: q, loading: false, error: (e as Error).message })
    }
  }

  function handlePick(candidate: ExternalStudio) {
    setState({
      step: 'edit',
      candidate,
      form: {
        source: candidate.source,
        externalId: candidate.externalId,
        name: candidate.name,
        overview: candidate.overview ?? '',
        contentType: 'adult',
        monitored: true,
        monitorMode: 'latest',
        parentExternalId: candidate.parentExternalId,
        parentName: candidate.parentName,
        parentImageUrl: candidate.parentImageUrl,
        parentWebsiteUrl: candidate.parentWebsiteUrl,
        imageUrl: candidate.imageUrl,
        websiteUrl: candidate.websiteUrl,
      },
    })
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
          <h2 className="text-sm font-semibold text-white">Add Studio</h2>
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
                  <StudioResult key={`${s.source}-${s.externalId}-${i}`} studio={s} onPick={handlePick} />
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
              <span className="text-sm">Adding site…</span>
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
              Add Studio
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
      <p className="text-xs text-white/40 mb-3">Search StashDB for a studio to add to your library.</p>
      <form onSubmit={e => { e.preventDefault(); onSearch(q) }} className="flex gap-2">
        <input
          autoFocus
          value={q}
          onChange={e => setQ(e.target.value)}
          placeholder="e.g. Bratty Sis"
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

function StudioResult({ studio, onPick }: { studio: ExternalStudio; onPick: (s: ExternalStudio) => void }) {
  return (
    <button
      onClick={() => onPick(studio)}
      className="w-full flex items-center gap-3 px-3 py-2.5 rounded-lg hover:bg-white/5 transition-colors text-left group"
    >
      {studio.imageUrl ? (
        <img src={studio.imageUrl} alt={studio.name} className="w-12 h-8 object-contain rounded shrink-0 bg-white/5" />
      ) : (
        <div className="w-12 h-8 rounded shrink-0 bg-white/5 flex items-center justify-center">
          <Building2 size={14} className="text-white/20" />
        </div>
      )}
      <div className="flex-1 min-w-0">
        <p className="text-sm font-medium text-white truncate">{studio.name}</p>
        {studio.parentName && (
          <p className="text-xs text-white/35 truncate">{studio.parentName}</p>
        )}
      </div>
      <span className="text-xs text-white/25 shrink-0 uppercase tracking-wide">{studio.source}</span>
      <ChevronRight size={14} className="text-white/20 group-hover:text-white/50 transition-colors shrink-0" />
    </button>
  )
}

function EditForm({ form, onChange }: { form: ImportStudioRequest; onChange: (f: ImportStudioRequest) => void }) {
  return (
    <div className="space-y-4">
      {/* Logo preview */}
      {form.imageUrl && (
        <div className="flex items-center gap-3">
          <img src={form.imageUrl} alt={form.name} className="h-12 max-w-[8rem] object-contain rounded bg-white/5 p-1" />
          <p className="text-xs text-white/35">Logo from {form.source}</p>
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

      {/* Website */}
      <div>
        <label className="block text-xs text-white/40 mb-1">Website</label>
        <input
          value={form.websiteUrl ?? ''}
          onChange={e => onChange({ ...form, websiteUrl: e.target.value })}
          className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-sm text-white outline-none focus:border-white/20"
          placeholder="https://…"
        />
      </div>

      {/* Network */}
      {form.parentName && (
        <div>
          <label className="block text-xs text-white/40 mb-1">Network (auto-created)</label>
          <p className="text-sm text-white/60 px-3 py-2 bg-white/3 rounded-lg border border-white/8">{form.parentName}</p>
        </div>
      )}

      {/* Monitor mode */}
      <div>
        <label className="block text-xs text-white/40 mb-1">Import mode</label>
        <select
          value={form.monitorMode}
          onChange={e => onChange({ ...form, monitorMode: e.target.value as ImportStudioRequest['monitorMode'] })}
          className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-sm text-white outline-none focus:border-white/20"
        >
          <option value="latest">Latest only — import all scenes; only the most recent is wanted</option>
          <option value="all">All — mark every scene as wanted</option>
          <option value="future">Future — only scenes released after today are wanted</option>
          <option value="none">None — import scenes without marking any as wanted</option>
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

export function StudiosPage() {
  const [search, setSearch] = useState('')
  const [offset, setOffset] = useState(0)
  const [showAdd, setShowAdd] = useState(false)

  const resetPage = (v: string) => { setSearch(v); setOffset(0) }

  const studios = useLibraryEntries({
    kind: 'studio',
    search: search || undefined,
    limit: LIMIT,
    offset,
  })

  return (
    <div>
      <PageHeader
        title="Studios"
        accent={ACCENT}
        search={search}
        onSearch={resetPage}
        total={studios.data?.total}
      >
        <button
          onClick={() => setShowAdd(true)}
          className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium text-white transition-colors shrink-0"
          style={{ background: ACCENT }}
        >
          <Plus size={14} />
          Add Studio
        </button>
      </PageHeader>

      <div className="px-8 py-6">
        {studios.isLoading ? (
          <SkeletonGrid count={24} aspect="16/9" />
        ) : !studios.data?.data.length ? (
          <EmptyState icon={Building2} title="No studios yet" accent={ACCENT} />
        ) : (
          <>
            <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-4">
              {studios.data.data.map(e => (
                <EntryCard key={e.id} entry={e} href={`/afterdark/studios/${e.id}`} aspect="16/9" accent={ACCENT} />
              ))}
            </div>
            <Pagination total={studios.data.total} limit={LIMIT} offset={offset} onChange={setOffset} accent={ACCENT} />
          </>
        )}
      </div>

      {showAdd && (
        <AddStudioDialog
          onClose={() => setShowAdd(false)}
          onImported={() => {
            setShowAdd(false)
            studios.refetch()
          }}
        />
      )}
    </div>
  )
}
