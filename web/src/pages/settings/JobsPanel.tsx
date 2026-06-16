import { useQueryClient } from '@tanstack/react-query'
import { X, CheckCircle2, AlertCircle, Clock, Loader2, Ban } from 'lucide-react'
import { useJobs, cancelJob } from '../../api/jobs'
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
  const isActive = status === 'queued' || status === 'running'

  if (!isActive && total === 0) return null

  if (total === 0) {
    return (
      <div className="w-full h-1 rounded-full bg-white/5 overflow-hidden">
        <div className="h-full w-full bg-blue-500/40 animate-pulse rounded-full" />
      </div>
    )
  }

  const pct = Math.min(100, Math.round((current / total) * 100))
  const barColor = status === 'failed' ? 'bg-red-500/50'
    : status === 'completed' ? 'bg-emerald-500/50'
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

function JobRow({ job, onCancel }: { job: Job; onCancel: (id: string) => void }) {
  const showCancel = job.status === 'running'
  const pct = job.total > 0 ? Math.min(100, Math.round((job.current / job.total) * 100)) : null

  return (
    <div className="flex items-center gap-4 py-3 border-b border-white/5 last:border-0">
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

      <div className="flex items-center gap-3 shrink-0">
        {pct !== null && (
          <span className="text-xs text-white/30 font-mono w-8 text-right">{pct}%</span>
        )}
        <StatusBadge status={job.status} />
        {showCancel && (
          <button
            onClick={() => onCancel(job.id)}
            className="flex items-center justify-center w-6 h-6 rounded-md text-white/30
              hover:text-white/70 hover:bg-white/5 transition-all"
            title="Cancel job"
          >
            <X size={13} />
          </button>
        )}
        {!showCancel && <div className="w-6" />}
      </div>
    </div>
  )
}

export function JobsPanel() {
  const { data, isLoading } = useJobs()
  const queryClient = useQueryClient()

  const jobs = data?.data ?? []

  async function handleCancel(id: string) {
    try {
      await cancelJob(id)
      queryClient.invalidateQueries({ queryKey: ['jobs'] })
    } catch {
      // poll will pick up the real state on the next tick
    }
  }

  return (
    <div className="px-8 py-10 max-w-3xl">
      <div className="mb-8">
        <h1 className="text-2xl font-semibold text-white">Background Jobs</h1>
        <p className="text-white/40 text-sm mt-1">
          Long-running tasks like metadata imports and library scans run here.
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
          <div className="px-4">
            {jobs.map(job => (
              <JobRow key={job.id} job={job} onCancel={handleCancel} />
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
