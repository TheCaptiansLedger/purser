import { useState } from 'react'
import { Search, X, Loader2 } from 'lucide-react'

export function canSearch(q: string): boolean {
  return q.trim().length > 0
}

type DialogState<TResult, TForm> =
  | { step: 'search'; query: string; loading: boolean; error?: string }
  | { step: 'results'; query: string; results: TResult[] }
  | { step: 'edit'; form: TForm; prevQuery: string; prevResults: TResult[] }
  | { step: 'saving' }

interface ImportDialogProps<TResult, TForm> {
  open: boolean
  onClose: () => void
  title: string
  accent: string
  searchHint?: string
  searchPlaceholder: string
  confirmLabel?: string
  savingLabel?: string
  keyOf: (item: TResult) => React.Key
  onSearch: (query: string) => Promise<TResult[]>
  buildForm: (item: TResult) => TForm
  renderResult: (item: TResult, onSelect: (item: TResult) => void) => React.ReactNode
  renderEditForm: (form: TForm, onChange: (form: TForm) => void) => React.ReactNode
  onImport: (form: TForm) => Promise<void>
}

function SearchStep({ initialQuery = '', hint, placeholder, accent, loading, error, onSearch }: {
  initialQuery?: string
  hint?: string
  placeholder: string
  accent: string
  loading: boolean
  error?: string
  onSearch: (q: string) => void
}) {
  const [q, setQ] = useState(initialQuery)
  return (
    <div>
      {hint && <p className="text-xs text-white/40 mb-3">{hint}</p>}
      <form onSubmit={e => { e.preventDefault(); onSearch(q) }} className="flex gap-2">
        <input
          autoFocus
          value={q}
          onChange={e => setQ(e.target.value)}
          placeholder={placeholder}
          className="flex-1 bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-sm text-white placeholder-white/25 outline-none focus:border-white/20"
        />
        <button
          type="submit"
          disabled={loading || !canSearch(q)}
          className="flex items-center gap-1.5 px-3 py-2 rounded-lg text-sm font-medium text-white disabled:opacity-40 transition-colors"
          style={{ background: accent }}
        >
          {loading ? <Loader2 size={14} className="animate-spin" /> : <Search size={14} />}
          Search
        </button>
      </form>
      {error && <p className="mt-2 text-xs text-red-400">{error}</p>}
    </div>
  )
}

export function ImportDialog<TResult, TForm>({
  open,
  onClose,
  title,
  accent,
  searchHint,
  searchPlaceholder,
  confirmLabel,
  savingLabel = 'Saving…',
  keyOf,
  onSearch,
  buildForm,
  renderResult,
  renderEditForm,
  onImport,
}: ImportDialogProps<TResult, TForm>) {
  const [state, setState] = useState<DialogState<TResult, TForm>>({
    step: 'search',
    query: '',
    loading: false,
  })

  if (!open) return null

  const label = confirmLabel ?? title

  async function handleSearch(q: string) {
    if (!canSearch(q)) return
    setState({ step: 'search', query: q, loading: true })
    try {
      const results = await onSearch(q)
      setState({ step: 'results', query: q, results })
    } catch (e) {
      setState({ step: 'search', query: q, loading: false, error: (e as Error).message })
    }
  }

  function handlePick(item: TResult) {
    const prevQuery = state.step === 'results' ? state.query : ''
    const prevResults = state.step === 'results' ? state.results : []
    setState({ step: 'edit', form: buildForm(item), prevQuery, prevResults })
  }

  async function handleSave() {
    if (state.step !== 'edit') return
    const { form, prevQuery, prevResults } = state
    setState({ step: 'saving' })
    try {
      await onImport(form)
      onClose()
    } catch {
      setState({ step: 'edit', form, prevQuery, prevResults })
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4" style={{ background: 'rgba(0,0,0,0.7)' }}>
      <div className="w-full max-w-lg xl:max-w-xl 2xl:max-w-2xl rounded-xl border border-white/10 shadow-2xl flex flex-col" style={{ background: '#0f0f17', maxHeight: '80vh' }}>

        <div className="flex items-center justify-between px-5 py-4 border-b border-white/8">
          <h2 className="text-sm font-semibold text-white">{title}</h2>
          <button onClick={onClose} className="text-white/40 hover:text-white/70 transition-colors">
            <X size={16} />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto px-5 py-4 min-h-0">

          {(state.step === 'search' || state.step === 'results') && (
            <SearchStep
              initialQuery={state.query}
              hint={searchHint}
              placeholder={searchPlaceholder}
              accent={accent}
              loading={state.step === 'search' && state.loading}
              error={state.step === 'search' ? state.error : undefined}
              onSearch={handleSearch}
            />
          )}

          {state.step === 'results' && (
            <div className="mt-4 space-y-1">
              {state.results.length === 0 ? (
                <p className="text-sm text-white/40 text-center py-6">No results found</p>
              ) : (
                state.results.map(item => (
                  <div key={keyOf(item)}>
                    {renderResult(item, handlePick)}
                  </div>
                ))
              )}
            </div>
          )}

          {state.step === 'edit' && renderEditForm(state.form, form => setState(s => s.step === 'edit' ? { ...s, form } : s))}

          {state.step === 'saving' && (
            <div className="flex items-center justify-center gap-3 py-12 text-white/50">
              <Loader2 size={18} className="animate-spin" />
              <span className="text-sm">{savingLabel}</span>
            </div>
          )}
        </div>

        {state.step === 'edit' && (
          <div className="px-5 py-4 border-t border-white/8 flex items-center justify-between gap-3">
            <button
              onClick={() => {
                if (state.step === 'edit') {
                  setState({ step: 'results', query: state.prevQuery, results: state.prevResults })
                }
              }}
              className="text-sm text-white/40 hover:text-white/70 transition-colors"
            >
              ← Back
            </button>
            <button
              onClick={() => { void handleSave() }}
              className="px-4 py-1.5 rounded-lg text-sm font-medium text-white transition-colors"
              style={{ background: accent }}
            >
              {label}
            </button>
          </div>
        )}
      </div>
    </div>
  )
}
