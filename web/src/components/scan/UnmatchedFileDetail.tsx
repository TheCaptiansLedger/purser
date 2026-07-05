import { useState } from 'react'
import { ChevronDown, ChevronRight, Film } from 'lucide-react'
import { fmtBytes } from '../ui/Runtime'
import type { UnmatchedFile, MatchCandidate } from '../../types'
import { manualMatch, dismissUnmatched } from '../../api/scan'
import { confidenceColor } from './UnmatchedFileCard'
import { ScrapeDialog } from './ScrapeDialog'
import { CreateFromCandidateDialog } from './CreateFromCandidateDialog'

interface Props {
  file: UnmatchedFile
  onResolved: () => void
}

function fmtDate(iso: string): string {
  return new Date(iso).toLocaleDateString(undefined, { year: 'numeric', month: 'long', day: 'numeric' })
}

function fmtDiscovered(iso: string): string {
  return new Date(iso).toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' })
}

function fmtRuntime(secs: number): string {
  const h = Math.floor(secs / 3600)
  const m = Math.floor((secs % 3600) / 60)
  if (h > 0) return `${h}h ${m}m`
  return `${m}m`
}

function ConfidenceBar({ confidence }: { confidence: number }) {
  const color = confidenceColor(confidence)
  return (
    <div className="flex items-center gap-2">
      <div className="flex-1 h-1 bg-white/10 rounded-full overflow-hidden">
        <div className="h-full rounded-full" style={{ width: `${confidence * 100}%`, background: color }} />
      </div>
      <span className="text-[10px] tabular-nums shrink-0 font-medium" style={{ color }}>
        {Math.round(confidence * 100)}%
      </span>
    </div>
  )
}

function CandidateRow({ candidate, onAccept }: { candidate: MatchCandidate; onAccept?: () => void }) {
  const title  = candidate.external?.title ?? candidate.item_title ?? '(unknown)'
  const parent = candidate.external?.parent?.name
  const isLocalMatch = Boolean(candidate.item_id)
  const conf   = candidate.confidence
  const color  = confidenceColor(conf)

  return (
    <div className="py-3 border-b border-white/4 last:border-0 space-y-2">
      {/* Source method badge + confidence */}
      <div className="flex items-center justify-between gap-2">
        <div className="flex items-center gap-2 min-w-0">
          <span
            className="text-[10px] font-medium px-1.5 py-0.5 rounded shrink-0"
            style={{ background: 'rgba(99,102,241,0.15)', color: '#818cf8' }}
          >
            {candidate.source_label}
          </span>
          {candidate.source_description && (
            <span className="text-[10px] text-white/30 truncate">{candidate.source_description}</span>
          )}
        </div>
        <span className="text-xs font-medium tabular-nums shrink-0" style={{ color }}>
          {Math.round(conf * 100)}%
        </span>
      </div>

      {/* Confidence bar */}
      <ConfidenceBar confidence={conf} />

      {/* Match title + parent */}
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <span className="text-xs text-white/70 block truncate">{title}</span>
          {parent && <span className="text-[10px] text-white/35">{parent}</span>}
        </div>
        {isLocalMatch && onAccept && (
          <button
            onClick={onAccept}
            className="shrink-0 text-[10px] font-medium px-2 py-0.5 rounded text-white transition-colors"
            style={{ background: '#10b981' }}
          >
            Accept
          </button>
        )}
      </div>
    </div>
  )
}

export function UnmatchedFileDetail({ file, onResolved }: Props) {
  const [accepting, setAccepting]   = useState(false)
  const [dismissing, setDismissing] = useState(false)
  const [scrapeOpen, setScrapeOpen] = useState(false)
  const [createCandidate, setCreateCandidate] = useState<MatchCandidate | null>(null)
  const [fingerprintOpen, setFingerprintOpen] = useState(false)

  const top = file.candidates[0]
  const ext = top?.external

  const imageUrl = ext?.image_url
  const title    = ext?.title ?? top?.item_title ?? file.path.split('/').pop() ?? file.path
  const studio   = ext?.parent?.name
  const overview = ext?.overview
  const date     = ext?.date
  const runtime  = ext?.runtime_seconds

  const canAccept = Boolean(top?.item_id)
  const canCreate = !canAccept && Boolean(ext?.parent)

  const handleAccept = async (itemId: string) => {
    setAccepting(true)
    try { await manualMatch(file.id, itemId); onResolved() }
    finally { setAccepting(false) }
  }

  const handleDismiss = async () => {
    setDismissing(true)
    try { await dismissUnmatched(file.id); onResolved() }
    finally { setDismissing(false) }
  }

  const tags = file.fingerprint?.embedded_tags ?? {}
  const hasTags = Object.keys(tags).length > 0

  return (
    <div className="p-6 space-y-5 max-w-2xl">
      {/* Thumbnail */}
      <div className="relative w-full rounded-xl overflow-hidden bg-white/3" style={{ aspectRatio: '16/9' }}>
        {imageUrl ? (
          <img src={imageUrl} alt={title} className="w-full h-full object-cover" />
        ) : (
          <div className="w-full h-full flex items-center justify-center">
            <Film size={32} className="text-white/10" strokeWidth={1} />
          </div>
        )}
      </div>

      {/* Title + meta */}
      <div>
        <h2 className="text-lg font-semibold text-white leading-tight">{title}</h2>
        {studio && <p className="text-sm text-white/50 mt-0.5">{studio}</p>}
        <div className="flex flex-wrap gap-3 mt-1.5 text-xs text-white/30">
          {date && <span>{fmtDate(date)}</span>}
          {runtime && runtime > 0 && <span>{fmtRuntime(runtime)}</span>}
          <span>{fmtBytes(file.size)}</span>
          <span>Found {fmtDiscovered(file.discovered_at)}</span>
        </div>
      </div>

      {/* Overview */}
      {overview && (
        <p className="text-sm text-white/55 leading-relaxed">{overview}</p>
      )}

      {/* Candidates */}
      {file.candidates.length > 0 && (
        <div>
          <p className="text-[10px] font-medium text-white/30 uppercase tracking-wider mb-1">
            {file.candidates.length} Candidate{file.candidates.length !== 1 ? 's' : ''}
          </p>
          <div>
            {file.candidates.map((c, i) => (
              <CandidateRow
                key={i}
                candidate={c}
                onAccept={c.item_id ? () => { void handleAccept(c.item_id) } : undefined}
              />
            ))}
          </div>
          {file.candidates.length === 0 && (
            <p className="text-xs text-white/25 italic">No candidates identified yet.</p>
          )}
        </div>
      )}

      {/* File details (collapsible) */}
      <div>
        <button
          onClick={() => setFingerprintOpen(v => !v)}
          className="flex items-center gap-1.5 text-[10px] font-medium text-white/30 uppercase tracking-wider hover:text-white/50 transition-colors"
        >
          {fingerprintOpen ? <ChevronDown size={11} /> : <ChevronRight size={11} />}
          File Details
        </button>
        {fingerprintOpen && (
          <div className="mt-2 rounded-lg bg-white/3 p-3 space-y-1.5 text-[10px] font-mono text-white/40">
            <p className="break-all"><span className="text-white/25">path: </span>{file.path}</p>
            {file.fingerprint?.oshash && (
              <p><span className="text-white/25">oshash: </span>{file.fingerprint.oshash}</p>
            )}
            {file.fingerprint?.phash && (
              <p><span className="text-white/25">phash: </span>{file.fingerprint.phash}</p>
            )}
            {file.fingerprint?.acoust_id && (
              <p><span className="text-white/25">acoustid: </span><span className="text-emerald-400/60">✓ computed</span></p>
            )}
            {hasTags && Object.entries(tags).map(([k, v]) => (
              <p key={k}><span className="text-white/25">{k}: </span>{v}</p>
            ))}
          </div>
        )}
      </div>

      {/* Actions */}
      <div className="flex items-center gap-2 flex-wrap pt-1 border-t border-white/5">
        {canAccept && (
          <button
            onClick={() => { void handleAccept(top!.item_id) }}
            disabled={accepting}
            className="text-sm font-medium px-4 py-2 rounded-lg text-white transition-colors disabled:opacity-50"
            style={{ background: '#10b981' }}
          >
            {accepting ? 'Accepting…' : 'Accept'}
          </button>
        )}
        {canCreate && (
          <button
            onClick={() => setCreateCandidate(top!)}
            className="text-sm font-medium px-4 py-2 rounded-lg text-white transition-colors"
            style={{ background: '#6366f1' }}
          >
            Import &amp; Create
          </button>
        )}
        <button
          onClick={() => setScrapeOpen(true)}
          className="text-sm font-medium px-4 py-2 rounded-lg border border-white/10 text-white/50 hover:text-white/80 hover:border-white/20 transition-colors"
        >
          Search Provider
        </button>
        <button
          onClick={() => { void handleDismiss() }}
          disabled={dismissing}
          className="text-sm font-medium px-4 py-2 rounded-lg border border-white/8 text-white/30 hover:text-white/55 hover:border-white/15 transition-colors disabled:opacity-50"
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
