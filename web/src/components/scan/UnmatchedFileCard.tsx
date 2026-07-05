import { useState } from 'react'
import { Film } from 'lucide-react'
import type { UnmatchedFile, MatchCandidate } from '../../types'
import { manualMatch, dismissUnmatched } from '../../api/scan'
import { ScrapeDialog } from './ScrapeDialog'
import { CreateFromCandidateDialog } from './CreateFromCandidateDialog'

interface Props {
  file: UnmatchedFile
  // Selectable mode (master-detail list): clicking the card calls onSelect, no inline actions.
  selected?: boolean
  onSelect?: () => void
  // Standalone mode (album group rows): card renders with inline action buttons.
  onResolved?: () => void
  compact?: boolean
}

export function confidenceColor(c: number): string {
  return c >= 0.85 ? '#10b981' : c >= 0.60 ? '#f59e0b' : '#ef4444'
}

function shortPath(path: string): string {
  const parts = path.split('/')
  return parts.length > 2 ? `…/${parts.slice(-2).join('/')}` : path
}

// ── Selectable compact card (master-detail left column) ───────────────────────

function SelectableCard({ file, selected, onSelect }: { file: UnmatchedFile; selected: boolean; onSelect: () => void }) {
  const top = file.candidates[0]
  const title  = top?.external?.title ?? top?.item_title ?? file.path.split('/').pop() ?? file.path
  const parent = top?.external?.parent?.name
  const imageUrl = top?.external?.image_url
  const conf   = top?.confidence ?? 0
  const color  = confidenceColor(conf)

  return (
    <button
      onClick={onSelect}
      className="w-full text-left flex items-center gap-3 px-4 py-3 border-b border-white/5 hover:bg-white/3 transition-colors"
      style={selected ? { background: 'rgba(99,102,241,0.08)', borderLeft: '2px solid #6366f1' } : { borderLeft: '2px solid transparent' }}
    >
      {/* Thumbnail */}
      <div className="shrink-0 w-16 h-10 rounded overflow-hidden bg-white/5 flex items-center justify-center">
        {imageUrl ? (
          <img src={imageUrl} alt="" className="w-full h-full object-cover" />
        ) : (
          <Film size={16} className="text-white/15" />
        )}
      </div>

      {/* Info */}
      <div className="flex-1 min-w-0">
        <p className="text-xs font-medium text-white/80 truncate">{title}</p>
        {parent && <p className="text-[10px] text-white/35 truncate">{parent}</p>}
        <p className="text-[10px] font-mono text-white/20 truncate mt-0.5">{shortPath(file.path)}</p>
      </div>

      {/* Confidence */}
      {top && (
        <span className="text-[10px] font-medium tabular-nums shrink-0" style={{ color }}>
          {Math.round(conf * 100)}%
        </span>
      )}
    </button>
  )
}

// ── Standalone card with inline actions (album group rows) ────────────────────

function StandaloneCard({ file, onResolved, compact = false }: { file: UnmatchedFile; onResolved: () => void; compact?: boolean }) {
  const [accepting, setAccepting]   = useState(false)
  const [dismissing, setDismissing] = useState(false)
  const [scrapeOpen, setScrapeOpen] = useState(false)
  const [createCandidate, setCreateCandidate] = useState<MatchCandidate | null>(null)

  const top      = file.candidates[0]
  const title    = top?.external?.title ?? top?.item_title
  const parent   = top?.external?.parent?.name
  const canAccept = Boolean(top?.item_id)
  const canCreate = !canAccept && Boolean(top?.external?.parent)
  const conf     = top?.confidence ?? 0

  const handleAccept = async () => {
    if (!top?.item_id) return
    setAccepting(true)
    try { await manualMatch(file.id, top.item_id); onResolved() }
    finally { setAccepting(false) }
  }

  const handleDismiss = async () => {
    setDismissing(true)
    try { await dismissUnmatched(file.id); onResolved() }
    finally { setDismissing(false) }
  }

  return (
    <div className={compact ? '' : 'rounded-xl border border-white/8 bg-white/2 p-4'}>
      {/* File path */}
      <p className="text-[10px] font-mono text-white/40 truncate mb-1" title={file.path}>{file.path}</p>

      {/* Top candidate summary */}
      {top && (
        <div className="flex items-center justify-between gap-2 mb-2">
          <div className="min-w-0">
            {title && <span className="text-xs text-white/75 truncate block">{title}</span>}
            {parent && <span className="text-[10px] text-white/35">{parent}</span>}
          </div>
          <span className="text-[10px] tabular-nums shrink-0" style={{ color: confidenceColor(conf) }}>
            {Math.round(conf * 100)}%
          </span>
        </div>
      )}

      {/* Actions */}
      <div className="flex items-center gap-1.5 flex-wrap">
        {canAccept && (
          <button
            onClick={() => { void handleAccept() }}
            disabled={accepting}
            className="text-xs font-medium px-2.5 py-1 rounded-lg text-white transition-colors disabled:opacity-50"
            style={{ background: '#10b981' }}
          >
            {accepting ? 'Accepting…' : 'Accept'}
          </button>
        )}
        {canCreate && (
          <button
            onClick={() => setCreateCandidate(top)}
            className="text-xs font-medium px-2.5 py-1 rounded-lg text-white transition-colors"
            style={{ background: '#6366f1' }}
          >
            Import &amp; Create
          </button>
        )}
        <button
          onClick={() => setScrapeOpen(true)}
          className="text-xs px-2.5 py-1 rounded-lg border border-white/10 text-white/45 hover:text-white/70 hover:border-white/20 transition-colors"
        >
          Search Provider
        </button>
        <button
          onClick={() => { void handleDismiss() }}
          disabled={dismissing}
          className="text-xs px-2.5 py-1 rounded-lg border border-white/8 text-white/25 hover:text-white/50 hover:border-white/15 transition-colors disabled:opacity-50"
        >
          {dismissing ? 'Dismissing…' : 'Dismiss'}
        </button>
      </div>

      {scrapeOpen && (
        <ScrapeDialog file={file} onMatch={onResolved} onClose={() => setScrapeOpen(false)} />
      )}
      {createCandidate && (
        <CreateFromCandidateDialog
          unmatchedId={file.id}
          file={file}
          candidate={createCandidate}
          onMatch={onResolved}
          onClose={() => setCreateCandidate(null)}
        />
      )}
    </div>
  )
}

// ── Public component ──────────────────────────────────────────────────────────

export function UnmatchedFileCard({ file, selected = false, onSelect, onResolved, compact = false }: Props) {
  if (onSelect) {
    return <SelectableCard file={file} selected={selected} onSelect={onSelect} />
  }
  return <StandaloneCard file={file} onResolved={onResolved!} compact={compact} />
}
