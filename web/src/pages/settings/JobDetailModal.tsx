import type { ReactNode } from 'react'
import { Modal } from '../../components/Modal'
import { JobStatusBadge } from '../../components/JobStatusBadge'
import { useWatchJob } from '../../hooks/useWatchJob'
import { jobFromProto } from './jobFromProto'
import { formatJobProgress, formatJobTimestamp } from './jobFormat'
import type { Job, Step, Task } from '../../types'

export interface JobDetailModalProps {
  jobId: string
  onClose: () => void
}

// JobDetailModal is #608's job detail modal: the full Task/Step tree,
// the Job's params and each Step's detail map, and all timestamps,
// live-updating via useWatchJob (#607) for as long as it's open. A modal
// dialog, per the issue's explicit rejection of the pre-reset docked side
// panel — built on the shared Modal primitive.
export function JobDetailModal({ jobId, onClose }: JobDetailModalProps) {
  const { job: protoJob, error } = useWatchJob(jobId)
  const job = protoJob ? jobFromProto(protoJob) : undefined

  return (
    <Modal title={job ? jobModalTitle(job) : 'Job detail'} onClose={onClose}>
      {/* Doherty threshold — see docs/design/ux-principles.md#feedback--system-status.
          The initial WatchJob event arrives well under 400ms over a local
          connection; a spinner here would read as slower, not more informative. */}
      {!job && !error && <p className="text-body text-text-secondary">Loading…</p>}
      {error && (
        <p className="text-status-failure text-body" role="alert">
          Couldn't load job ({error.message}).
        </p>
      )}
      {job && <JobDetail job={job} />}
    </Modal>
  )
}

function jobModalTitle(job: Job): string {
  return `${job.kind} — ${job.id}`
}

function JobDetail({ job }: { job: Job }) {
  return (
    <div className="flex flex-col gap-5">
      <div className="grid grid-cols-2 gap-x-4 gap-y-3 sm:grid-cols-3">
        <DetailField label="Status">
          <JobStatusBadge status={job.status} />
        </DetailField>
        <DetailField label="Progress">{formatJobProgress(job.progress)}</DetailField>
        <DetailField label="Created">{formatJobTimestamp(job.createdAt)}</DetailField>
        <DetailField label="Started">{formatJobTimestamp(job.startedAt)}</DetailField>
        <DetailField label="Finished">{formatJobTimestamp(job.finishedAt)}</DetailField>
      </div>
      <KeyValueSection title="Params" values={job.params} />
      <TaskTree tasks={job.tasks} />
    </div>
  )
}

function DetailField({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex flex-col gap-0.5">
      <span className="text-label text-text-secondary">{label}</span>
      <span className="text-body text-text">{children}</span>
    </div>
  )
}

// KeyValueSection renders a Record<string, string> as a labeled list —
// shared by Job.params and Step.detail (both opaque, kind/step-specific
// maps per docs/adr/0023-job-queue.md). Renders nothing for an empty map
// rather than an empty, misleading heading.
function KeyValueSection({ title, values }: { title: string; values: Record<string, string> }) {
  const entries = Object.entries(values)
  if (entries.length === 0) {
    return null
  }
  return (
    <div className="flex flex-col gap-1.5">
      <h3 className="text-label text-text-secondary">{title}</h3>
      <div className="flex flex-col gap-1 rounded-lg bg-surface p-2.5">
        {entries.map(([key, value]) => (
          <div key={key} className="flex gap-2 text-label">
            <span className="text-text-secondary">{key}</span>
            <span className="text-text">{value}</span>
          </div>
        ))}
      </div>
    </div>
  )
}

function TaskTree({ tasks }: { tasks: Task[] }) {
  if (tasks.length === 0) {
    return <p className="text-body text-text-secondary">No tasks yet.</p>
  }
  return (
    <div className="flex flex-col gap-2.5">
      <h3 className="text-label text-text-secondary">Tasks ({tasks.length})</h3>
      {tasks.map(task => (
        <TaskRow key={task.id} task={task} />
      ))}
    </div>
  )
}

function TaskRow({ task }: { task: Task }) {
  return (
    <div className="flex flex-col gap-2 rounded-lg border border-border p-3">
      <div className="flex items-center justify-between gap-3">
        <span className="text-body font-medium text-text">{task.label}</span>
        <div className="flex items-center gap-3">
          <JobStatusBadge status={task.status} />
          <span className="text-label text-text-secondary">{formatJobProgress(task.progress)}</span>
        </div>
      </div>
      <div className="flex gap-4 text-label text-text-secondary">
        <span>Started: {formatJobTimestamp(task.startedAt)}</span>
        <span>Finished: {formatJobTimestamp(task.finishedAt)}</span>
      </div>
      {task.steps.length > 0 && (
        <ul className="flex flex-col gap-2 border-t border-border pt-2">
          {task.steps.map(step => (
            <StepRow key={step.id} step={step} />
          ))}
        </ul>
      )}
    </div>
  )
}

function StepRow({ step }: { step: Step }) {
  return (
    <li className="flex flex-col gap-1">
      <div className="flex items-center justify-between gap-3">
        <span className="text-label text-text">{step.name}</span>
        <JobStatusBadge status={step.status} />
      </div>
      <div className="flex gap-4 text-label text-text-secondary">
        <span>Started: {formatJobTimestamp(step.startedAt)}</span>
        <span>Finished: {formatJobTimestamp(step.finishedAt)}</span>
      </div>
      {step.message && <p className="text-label text-text-secondary">{step.message}</p>}
      <KeyValueSection title="Detail" values={step.detail} />
    </li>
  )
}
