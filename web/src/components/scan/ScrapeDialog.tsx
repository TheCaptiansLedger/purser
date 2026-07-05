import { useState } from 'react'
import { X, Search, Loader2 } from 'lucide-react'
import type { UnmatchedFile, MatchCandidate } from '../../types'
import { rescrape, manualMatch } from '../../api/scan'

interface Props {
  file: UnmatchedFile
  onMatch: () => void
  onClose: () => void
}

function initialQuery(file: UnmatchedFile): string {
  const tags = file.fingerprint?.embedded_tags ?? {}
  const parts = [tags['title'], tags['artist']].filter(Boolean)
  if (parts.length > 0) return parts.join(' ')
  return file.path.split('/').pop()?.replace(/\.[^.]+$/, '') ?? ''
}

function ConfidencePip({ confidence }: { confidence: number }) {
  const color = confidence >= 0.85 ? '#10b981' : confidence >= 0.60 ? '#f59e0b' : '#ef4444'
  return (
    <span className="text-xs tabular-nums font-mono" style={{ color }}>
      {Math.round(confidence * 100)}%
    </span>
  )
}

export function ScrapeDialog({ file, onMatch, onClose }: Props) {
  const [query, setQuery]           = useState(initialQuery(file))
  const [loading, setLoading]       = useState(false)
  const [candidates, setCandidates] = useState<MatchCandidate[] | null>(null)
  const [error, setError]           = useState<string | null>(null)
  const [accepting, setAccepting]   = useState<string | null>(null)

  const handleSearch = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!query.trim()) return
    setLoading(true)
    setError(null)
    setCandidates(null)
    try {
      const res = await rescrape(file.id, query.trim())
      setCandidates(res.candidates)
    } catch {
      setError('Search failed — try a different query')
    } finally {
      setLoading(false)
    }
  }

  const handleSelect = async (candidate: MatchCandidate) => {
    if (!candidate.item_id) return
    setAccepting(candidate.item_id)
    try {
      await manualMatch(file.id, candidate.item_id)
      onMatch()
      onClose()
    } catch {
      setAccepting(null)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4" style={{ background: 'rgba(0,0,0,0.7)' }}>
      <div
        className="w-full max-w-lg rounded-xl border border-white/10 shadow-2xl flex flex-col"
        style={{ background: '#0f0f17', maxHeight: '80vh' }}
      >
        <div className="flex items-center justify-between px-5 py-4 border-b border-white/8">
          <div>
            <h2 className="text-sm font-semibold text-white">Search Provider</h2>
            <p className="text-[10px] text-white/35 font-mono truncate max-w-xs mt-0.5">{file.path}</p>
          </div>
          <button onClick={onClose} className="text-white/40 hover:text-white/70 transition-colors">
            <X size={16} />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto px-5 py-4 min-h-0 space-y-4">
          <form onSubmit={(e) => { void handleSearch(e) }} className="flex gap-2">
            <input
              autoFocus
              value={query}
              onChange={e => setQuery(e.target.value)}
              placeholder="Search title, artist…"
              className="flex-1 bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-sm text-white placeholder-white/25 outline-none focus:border-white/20"
            />
            <button
              type="submit"
              disabled={loading || !query.trim()}
              className="flex items-center gap-1.5 px-3 py-2 rounded-lg text-sm font-medium text-white disabled:opacity-40 transition-colors"
              style={{ background: '#6366f1' }}
            >
              {loading ? <Loader2 size={14} className="animate-spin" /> : <Search size={14} />}
              Search
            </button>
          </form>

          {error && <p className="text-xs text-red-400">{error}</p>}

          {candidates !== null && (
            candidates.length === 0 ? (
              <div className="text-center py-8">
                <p className="text-sm text-white/40">No matches found</p>
                <p className="text-xs text-white/25 mt-1">Try a different search, or add the item to your library first then retry.</p>
              </div>
            ) : (
              <div className="space-y-1">
                {candidates.map((c, i) => (
                  <button
                    key={c.item_id || i}
                    disabled={!c.item_id || accepting !== null}
                    onClick={() => { void handleSelect(c) }}
                    className="w-full flex items-center justify-between gap-3 px-3 py-2.5 rounded-lg bg-white/3 hover:bg-white/6 disabled:opacity-50 disabled:cursor-default transition-colors text-left"
                  >
                    <div className="min-w-0">
                      <p className="text-sm text-white/80 truncate">{c.item_title || c.external?.title || '(unknown)'}</p>
                      {c.external?.parent?.name && (
                        <p className="text-xs text-white/35 truncate">{c.external.parent.name}</p>
                      )}
                    </div>
                    <div className="flex items-center gap-2 shrink-0">
                      <ConfidencePip confidence={c.confidence} />
                      {accepting === c.item_id && <Loader2 size={12} className="animate-spin text-white/50" />}
                    </div>
                  </button>
                ))}
              </div>
            )
          )}
        </div>
      </div>
    </div>
  )
}
