import { useState } from 'react'
import { ChevronDown, ChevronRight } from 'lucide-react'
import type { UnmatchedFileGroup } from '../../types'
import { UnmatchedFileCard } from './UnmatchedFileCard'
import { dismissUnmatched, manualMatch } from '../../api/scan'

interface Props {
  group: UnmatchedFileGroup
  onResolved: () => void
}

function ConfidenceColor(confidence: number): string {
  return confidence >= 0.85 ? '#10b981' : confidence >= 0.60 ? '#f59e0b' : '#ef4444'
}

export function AlbumUnmatchedGroup({ group, onResolved }: Props) {
  const [expanded, setExpanded]       = useState(true)
  const [acceptingAll, setAcceptingAll] = useState(false)
  const [dismissingAll, setDismissingAll] = useState(false)

  const resolved = group.files.filter(f => f.status !== 'pending').length
  const total    = group.files.length

  const allHaveItems = group.files.every(f => f.candidates[0]?.item_id)

  const handleAcceptAll = async () => {
    if (!allHaveItems) return
    setAcceptingAll(true)
    try {
      await Promise.all(
        group.files.map(f => manualMatch(f.id, f.candidates[0].item_id))
      )
      onResolved()
    } finally {
      setAcceptingAll(false)
    }
  }

  const handleDismissAll = async () => {
    setDismissingAll(true)
    try {
      await Promise.all(group.files.map(f => dismissUnmatched(f.id)))
      onResolved()
    } finally {
      setDismissingAll(false)
    }
  }

  return (
    <div className="rounded-xl border border-white/8 overflow-hidden">
      {/* Group header */}
      <div className="flex items-center gap-3 px-4 py-3 bg-white/3 border-b border-white/5">
        <button
          onClick={() => setExpanded(v => !v)}
          className="text-white/40 hover:text-white/70 transition-colors shrink-0"
          aria-label={expanded ? 'Collapse' : 'Expand'}
        >
          {expanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
        </button>

        <div className="flex-1 min-w-0">
          <p className="text-sm font-medium text-white/80 truncate">{group.group_title || 'Unknown Album'}</p>
          <p className="text-[10px] text-white/30 mt-0.5">
            {total} track{total !== 1 ? 's' : ''}
            {resolved > 0 && ` · ${resolved} resolved`}
          </p>
        </div>

        <span
          className="text-xs font-mono shrink-0"
          style={{ color: ConfidenceColor(group.best_candidate_confidence) }}
        >
          {Math.round(group.best_candidate_confidence * 100)}%
        </span>

        <div className="flex items-center gap-1.5 shrink-0">
          {allHaveItems && (
            <button
              onClick={() => { void handleAcceptAll() }}
              disabled={acceptingAll || dismissingAll}
              className="text-xs font-medium px-2.5 py-1 rounded-lg text-white disabled:opacity-50 transition-colors"
              style={{ background: '#10b981' }}
            >
              {acceptingAll ? 'Accepting…' : 'Accept All'}
            </button>
          )}
          <button
            onClick={() => { void handleDismissAll() }}
            disabled={acceptingAll || dismissingAll}
            className="text-xs font-medium px-2.5 py-1 rounded-lg border border-white/8 text-white/35 hover:text-white/55 hover:border-white/15 disabled:opacity-50 transition-colors"
          >
            {dismissingAll ? 'Dismissing…' : 'Dismiss All'}
          </button>
        </div>
      </div>

      {/* Track list */}
      {expanded && (
        <div className="divide-y divide-white/4">
          {group.files.map(f => (
            <div key={f.id} className="px-4 py-3">
              <UnmatchedFileCard file={f} onResolved={onResolved} compact />
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
