import { useState } from 'react'
import { Building2, Plus, ChevronRight } from 'lucide-react'
import { useQueryClient } from '@tanstack/react-query'
import { useLibraryEntries } from '../../api/library'
import { searchStudios, importStudio } from '../../api/metadata'
import type { ImportStudioRequest } from '../../api/metadata'
import type { ExternalStudio } from '../../types'
import { PageHeader } from '../../components/layout/PageHeader'
import { EntryCard } from '../../components/media/EntryCard'
import { Pagination } from '../../components/ui/Pagination'
import { SkeletonGrid } from '../../components/ui/Skeleton'
import { EmptyState } from '../../components/ui/EmptyState'
import { ImportDialog } from '../../components/ImportDialog'

const ACCENT = '#f43f5e'
const LIMIT = 48

function studioImportRequest(candidate: ExternalStudio): ImportStudioRequest {
  return {
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
  }
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

function StudioEditForm({ form, onChange }: { form: ImportStudioRequest; onChange: (f: ImportStudioRequest) => void }) {
  return (
    <div className="space-y-4">
      {form.imageUrl && (
        <div className="flex items-center gap-3">
          <img src={form.imageUrl} alt={form.name} className="h-12 max-w-[8rem] object-contain rounded bg-white/5 p-1" />
          <p className="text-xs text-white/35">Logo from {form.source}</p>
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
        <label className="block text-xs text-white/40 mb-1">Website</label>
        <input
          value={form.websiteUrl ?? ''}
          onChange={e => onChange({ ...form, websiteUrl: e.target.value })}
          className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-sm text-white outline-none focus:border-white/20"
          placeholder="https://…"
        />
      </div>

      {form.parentName && (
        <div>
          <label className="block text-xs text-white/40 mb-1">Network (auto-created)</label>
          <p className="text-sm text-white/60 px-3 py-2 bg-white/3 rounded-lg border border-white/8">{form.parentName}</p>
        </div>
      )}

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

export function StudiosPage() {
  const queryClient = useQueryClient()
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

  async function handleImport(form: ImportStudioRequest) {
    await importStudio(form)
    queryClient.invalidateQueries({ queryKey: ['library-entries'] })
  }

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
            <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 2xl:grid-cols-6 gap-4">
              {studios.data.data.map(e => (
                <EntryCard key={e.id} entry={e} href={`/afterdark/studios/${e.id}`} aspect="16/9" accent={ACCENT} />
              ))}
            </div>
            <Pagination total={studios.data.total} limit={LIMIT} offset={offset} onChange={setOffset} accent={ACCENT} />
          </>
        )}
      </div>

      <ImportDialog<ExternalStudio, ImportStudioRequest>
        open={showAdd}
        onClose={() => setShowAdd(false)}
        title="Add Studio"
        accent={ACCENT}
        searchHint="Search StashDB for a studio to add to your library."
        searchPlaceholder="e.g. Bratty Sis"
        savingLabel="Adding site…"
        keyOf={s => `${s.source}-${s.externalId}`}
        onSearch={q => searchStudios(q, 'adult').then(r => r.results)}
        buildForm={studioImportRequest}
        renderResult={(item, onSelect) => <StudioResult studio={item} onPick={onSelect} />}
        renderEditForm={(form, onChange) => <StudioEditForm form={form} onChange={onChange} />}
        onImport={handleImport}
      />
    </div>
  )
}
