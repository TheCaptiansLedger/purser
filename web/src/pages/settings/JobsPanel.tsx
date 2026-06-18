import { useState, useEffect } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { X, CheckCircle2, AlertCircle, Clock, Loader2, Ban } from 'lucide-react'
import { useJobs, cancelJob, JOBS_DISPLAY_LIMIT } from '../../api/jobs'
import type { Job, JobStatus } from '../../types'

const STATUS_META: Record<JobStatus, { label: string; color: string; icon: React.ReactNode }> = {
  queued:    { label: 'Queued',    color: 'text-white/40',    icon: <Clock size={13} /> },
  running:   { label: 'Running',   color: 'text-blue-400',    icon: <Loader2 size={13} className="animate-spin" /> },
  completed: { label: 'Done',      color: 'text-emerald-400', icon: <CheckCircle2 size={13} /> },
  failed:    { label: 'Failed',    color: 'text-red-400',     icon: <AlertCircle size={13} /> },
  cancelled: { label: 'Cancelled', color: 'text-white/30',    icon: <Ban size={13} /> },
}

function StatusBadge({ status }: { status: JobStatus }) {
  const { label, color, icon } = STATUS_META[status]
  return (
    <span className={`inline-flex items-center gap-1.5 text-xs font-mono ${color}`}>
      {icon}
      {label}
    </span>
  )
}

function ProgressBar({ current, total, status }: { current: number; total: number; status: JobStatus }) {
  if (total === 0) {
    if (status === 'running') {
      return (
        <div className="w-full h-1 rounded-full bg-white/5 overflow-hidden">
          <div className="h-full w-full bg-blue-500/40 animate-pulse rounded-full" />
        </div>
      )
    }
    if (status === 'completed') {
      return (
        <div className="w-full h-1 rounded-full bg-white/5 overflow-hidden">
          <div className="h-full w-full bg-emerald-500/40 rounded-full" />
        </div>
      )
    }
    if (status === 'failed') {
      return (
        <div className="w-full h-1 rounded-full bg-white/5 overflow-hidden">
          <div className="h-full w-full bg-red-500/40 rounded-full" />
        </div>
      )
    }
    return null
  }

  const pct = Math.min(100, Math.round((current / total) * 100))
  const barColor = status === 'failed'    ? 'bg-red-500/50'
    : status === 'completed' ? 'bg-emerald-500/50'
    : status === 'cancelled' ? 'bg-white/10'
    : 'bg-blue-500/50'

  return (
    <div className="w-full h-1 rounded-full bg-white/5 overflow-hidden">
      <div
        className={`h-full rounded-full transition-all duration-300 ${barColor}`}
        style={{ width: `${pct}%` }}
      />
    </div>
  )
}

export function fmtTime(iso: string): string {
  const d = new Date(iso)
  const now = new Date()
  const sameDay = d.toDateString() === now.toDateString()
  return sameDay
    ? d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
    : d.toLocaleDateString([], { month: 'short', day: 'numeric' }) + ' ' +
      d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

export function jobTimestamp(job: Job): { label: string; ts: string } | null {
  if (job.status === 'queued') return { label: 'queued', ts: job.createdAt }
  if (job.status === 'running' && job.startedAt) return { label: 'started', ts: job.startedAt }
  if (job.completedAt) return { label: 'finished', ts: job.completedAt }
  return null
}

function JobRow({
  job,
  onCancel,
  isSelected,
  onClick,
}: {
  job: Job
  onCancel: (id: string) => void
  isSelected: boolean
  onClick: () => void
}) {
  const showCancel = job.status === 'running' || job.status === 'queued'
  const pct = job.total > 0 ? Math.min(100, Math.round((job.current / job.total) * 100)) : null
  const stamp = jobTimestamp(job)

  return (
    <div
      onClick={onClick}
      className={[
        'flex items-center gap-4 py-3 border-b border-white/5 last:border-0 cursor-pointer transition-colors',
        isSelected ? 'bg-white/5' : 'hover:bg-white/[0.03]',
      ].join(' ')}
    >
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 mb-1.5">
          <span className="text-sm text-white/80 font-mono truncate">{job.name}</span>
          {job.message && (
            <span className="text-xs text-white/30 truncate hidden sm:block">{job.message}</span>
          )}
        </div>
        <ProgressBar current={job.current} total={job.total} status={job.status} />
        {job.error && (
          <p className="text-xs text-red-400/80 mt-1 truncate">{job.error}</p>
        )}
      </div>

      <div className="flex flex-col items-end gap-1 shrink-0">
        <div className="flex items-center gap-3">
          {pct !== null && (
            <span className="text-xs text-white/30 font-mono w-8 text-right">{pct}%</span>
          )}
          <StatusBadge status={job.status} />
          {showCancel && (
            <button
              onClick={e => { e.stopPropagation(); onCancel(job.id) }}
              className="flex items-center justify-center w-6 h-6 rounded-md text-white/30
                hover:text-white/70 hover:bg-white/5 transition-all"
              title="Cancel job"
            >
              <X size={13} />
            </button>
          )}
          {!showCancel && <div className="w-6" />}
        </div>
        {stamp && (
          <span className="text-[10px] text-white/20 font-mono">
            {stamp.label} {fmtTime(stamp.ts)}
          </span>
        )}
      </div>
    </div>
  )
}

function JobDetailPanel({ job, onClose }: { job: Job; onClose: () => void }) {
  const pct = job.total > 0 ? Math.min(100, Math.round((job.current / job.total) * 100)) : null

  return (
    <div className="flex flex-col border-l border-white/5 bg-white/[0.015]">
      <div className="flex items-center justify-between px-5 py-4 border-b border-white/5 shrink-0">
        <span className="text-[10px] font-mono text-white/30 uppercase tracking-widest">Detail</span>
        <button
          onClick={onClose}
          className="flex items-center justify-center w-6 h-6 rounded-md text-white/30 hover:text-white/60 hover:bg-white/5 transition-all"
          aria-label="Close panel"
        >
          <X size={13} />
        </button>
      </div>

      <div className="overflow-y-auto px-5 py-5 space-y-5">
        <div>
          <p className="text-[10px] font-mono text-white/30 uppercase tracking-widest mb-1">Name</p>
          <p className="text-sm font-mono text-white/80 break-all">{job.name}</p>
        </div>

        <div>
          <p className="text-[10px] font-mono text-white/30 uppercase tracking-widest mb-2">Status</p>
          <StatusBadge status={job.status} />
        </div>

        <div>
          <p className="text-[10px] font-mono text-white/30 uppercase tracking-widest mb-2">
            Progress{pct !== null ? ` — ${pct}%` : ''}
          </p>
          <ProgressBar current={job.current} total={job.total} status={job.status} />
          {job.total > 0 && (
            <p className="text-[10px] font-mono text-white/25 mt-1.5">
              {job.current} / {job.total}
            </p>
          )}
        </div>

        {job.message && (
          <div>
            <p className="text-[10px] font-mono text-white/30 uppercase tracking-widest mb-1">Log</p>
            <p className="text-sm text-white/60">{job.message}</p>
          </div>
        )}

        {job.error && (
          <div>
            <p className="text-[10px] font-mono text-white/30 uppercase tracking-widest mb-1">Error</p>
            <p className="text-sm text-red-400/80 break-words">{job.error}</p>
          </div>
        )}

        <div>
          <p className="text-[10px] font-mono text-white/30 uppercase tracking-widest mb-2">Timestamps</p>
          <div className="space-y-1.5">
            <div className="flex justify-between text-xs font-mono">
              <span className="text-white/30">Created</span>
              <span className="text-white/50">{fmtTime(job.createdAt)}</span>
            </div>
            {job.startedAt && (
              <div className="flex justify-between text-xs font-mono">
                <span className="text-white/30">Started</span>
                <span className="text-white/50">{fmtTime(job.startedAt)}</span>
              </div>
            )}
            {job.completedAt && (
              <div className="flex justify-between text-xs font-mono">
                <span className="text-white/30">Finished</span>
                <span className="text-white/50">{fmtTime(job.completedAt)}</span>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

export function JobsPanel() {
  const { data, isLoading } = useJobs()
  const queryClient = useQueryClient()
  const [selectedId, setSelectedId] = useState<string | null>(null)

  const allJobs = data?.data ?? []
  const activeJobs = allJobs
    .filter(j => j.status === 'running' || j.status === 'queued')
    .sort((a, b) => (a.status === 'running' ? 0 : 1) - (b.status === 'running' ? 0 : 1))
  const terminalJobs = allJobs.filter(j => j.status !== 'running' && j.status !== 'queued')
  const terminalSlots = Math.max(0, JOBS_DISPLAY_LIMIT - activeJobs.length)
  const jobs = [...activeJobs, ...terminalJobs.slice(0, terminalSlots)]

  const selectedJob = selectedId ? (allJobs.find(j => j.id === selectedId) ?? null) : null
  const panelOpen = selectedJob !== null

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') setSelectedId(null)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  async function handleCancel(id: string) {
    try {
      await cancelJob(id)
      queryClient.invalidateQueries({ queryKey: ['jobs'] })
    } catch {
      // poll will pick up the real state on the next tick
    }
  }

  return (
    <div className="flex">
      {/* Job list — full width when no panel, 40% when panel open */}
      <div className={['min-w-0 transition-all duration-200', panelOpen ? 'w-2/5' : 'w-full'].join(' ')}>
        <div className="px-8 py-10">
          <div className="mb-8">
            <h1 className="text-2xl font-semibold text-white">Background Jobs</h1>
            <p className="text-white/40 text-sm mt-1">
              All active jobs + last {JOBS_DISPLAY_LIMIT} completed — updates every 2s.
            </p>
          </div>

          <div className="rounded-xl border border-white/5 bg-white/2">
            {isLoading ? (
              <div className="px-4 py-3 space-y-4">
                {Array.from({ length: 3 }).map((_, i) => (
                  <div key={i} className="space-y-2">
                    <div className="h-3 w-48 bg-white/5 rounded animate-pulse" />
                    <div className="h-1 w-full bg-white/5 rounded-full animate-pulse" />
                  </div>
                ))}
              </div>
            ) : jobs.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-16 text-center">
                <Clock size={28} className="text-white/10 mb-3" />
                <p className="text-sm text-white/25 font-mono">no jobs yet</p>
              </div>
            ) : (
              <div className="px-4 max-h-[32rem] overflow-y-auto">
                {jobs.map(job => (
                  <JobRow
                    key={job.id}
                    job={job}
                    onCancel={handleCancel}
                    isSelected={selectedId === job.id}
                    onClick={() => setSelectedId(id => (id === job.id ? null : job.id))}
                  />
                ))}
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Detail panel — flex-1 fills the remaining 60% */}
      {panelOpen && selectedJob && (
        <div className="flex-1 min-w-0 sticky top-0 self-start">
          <JobDetailPanel job={selectedJob} onClose={() => setSelectedId(null)} />
        </div>
      )}
    </div>
  )
}
