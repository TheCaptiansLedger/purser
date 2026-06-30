import { useState } from 'react'
import { X, Search, Loader2, ChevronRight, User } from 'lucide-react'
import { searchPeople } from '../../../api/metadata'
import { canSearch } from '../../ImportDialog'
import type { ExternalPerson, PersonRole } from '../../../types'

type Step =
  | { tag: 'search'; query: string; loading: boolean; error?: string }
  | { tag: 'results'; query: string; results: ExternalPerson[] }

interface PersonScrapeDialogProps {
  open: boolean
  initialQuery: string
  roles?: PersonRole[]
  onClose: () => void
  onApply: (person: ExternalPerson) => void
}

export function PersonScrapeDialog({ open, initialQuery, roles, onClose, onApply }: PersonScrapeDialogProps) {
  const [step, setStep] = useState<Step>({ tag: 'search', query: initialQuery, loading: false })
  const [query, setQuery] = useState(initialQuery)

  if (!open) return null

  function handleClose() {
    setStep({ tag: 'search', query: initialQuery, loading: false })
    setQuery(initialQuery)
    onClose()
  }

  async function handleSearch(q: string) {
    if (!canSearch(q)) return
    setStep({ tag: 'search', query: q, loading: true })
    const role = roles?.length === 1 ? roles[0] : undefined
    try {
      const results = await searchPeople(q, role).then(r => r.results)
      setStep({ tag: 'results', query: q, results })
    } catch (e) {
      setStep({ tag: 'search', query: q, loading: false, error: (e as Error).message })
    }
  }

  function handlePick(person: ExternalPerson) {
    onApply(person)
    handleClose()
  }

  const loading = step.tag === 'search' && step.loading
  const error = step.tag === 'search' ? step.error : undefined

  return (
    <div className="fixed inset-0 z-60 flex items-center justify-center p-4" style={{ background: 'rgba(0,0,0,0.6)' }}>
      <div className="w-full max-w-md rounded-xl border border-white/10 shadow-2xl flex flex-col" style={{ background: '#0f0f17', maxHeight: '75vh' }}>

        <div className="flex items-center justify-between px-5 py-4 border-b border-white/8">
          <h2 className="text-sm font-semibold text-white">Scrape metadata</h2>
          <button onClick={handleClose} className="text-white/40 hover:text-white/70 transition-colors">
            <X size={16} />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto px-5 py-4 min-h-0">
          <p className="text-xs text-white/40 mb-3">Search external metadata sources. Selecting a result merges its data into the form.</p>

          <form onSubmit={e => { e.preventDefault(); void handleSearch(query) }} className="flex gap-2 mb-4">
            <input
              autoFocus
              value={query}
              onChange={e => setQuery(e.target.value)}
              placeholder="Search by name…"
              className="flex-1 bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-sm text-white placeholder-white/25 outline-none focus:border-white/20"
            />
            <button
              type="submit"
              disabled={loading || !canSearch(query)}
              className="flex items-center gap-1.5 px-3 py-2 rounded-lg text-sm font-medium text-white bg-white/10 hover:bg-white/15 disabled:opacity-40 transition-colors"
            >
              {loading ? <Loader2 size={14} className="animate-spin" /> : <Search size={14} />}
              Search
            </button>
          </form>

          {error && <p className="mb-3 text-xs text-red-400">{error}</p>}

          {step.tag === 'results' && (
            <div className="space-y-1">
              {step.results.length === 0 ? (
                <p className="text-sm text-white/40 text-center py-6">No results found</p>
              ) : (
                step.results.map(p => (
                  <button
                    key={`${p.source}-${p.externalId}`}
                    onClick={() => handlePick(p)}
                    className="w-full flex items-center gap-3 px-3 py-2.5 rounded-lg hover:bg-white/5 transition-colors text-left group"
                  >
                    {p.imageUrl ? (
                      <img src={p.imageUrl} alt={p.name} className="w-9 h-9 object-cover rounded-full shrink-0 bg-white/5" />
                    ) : (
                      <div className="w-9 h-9 rounded-full shrink-0 bg-white/5 flex items-center justify-center">
                        <User size={13} className="text-white/20" />
                      </div>
                    )}
                    <div className="flex-1 min-w-0">
                      <p className="text-sm font-medium text-white truncate">{p.name}</p>
                      {p.aliases?.length ? (
                        <p className="text-xs text-white/35 truncate">{p.aliases.slice(0, 2).join(', ')}</p>
                      ) : null}
                    </div>
                    <span className="text-xs text-white/25 shrink-0 uppercase tracking-wide">{p.source}</span>
                    <ChevronRight size={14} className="text-white/20 group-hover:text-white/50 transition-colors shrink-0" />
                  </button>
                ))
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
