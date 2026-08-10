import { timestampDate } from '@bufbuild/protobuf/wkt'
import { JobStatus as ProtoJobStatus } from '../../gen/purser/job/v1/job_pb'
import type { Job as ProtoJob, Step as ProtoStep, Task as ProtoTask } from '../../gen/purser/job/v1/job_pb'
import type { Job, JobStatus, Step, Task } from '../../types'

const STATUS_BY_PROTO: Record<ProtoJobStatus, JobStatus> = {
  [ProtoJobStatus.UNSPECIFIED]: 'unspecified',
  [ProtoJobStatus.PENDING]: 'pending',
  [ProtoJobStatus.RUNNING]: 'running',
  [ProtoJobStatus.SUCCEEDED]: 'succeeded',
  [ProtoJobStatus.FAILED]: 'failed',
  [ProtoJobStatus.PARTIAL]: 'partial',
}

// stepFromProto converts one wire purser.job.v1.Step. detail is already a
// plain string-keyed object on the generated map field, so it's carried
// through as-is.
function stepFromProto(step: ProtoStep): Step {
  return {
    id: step.id,
    name: step.name,
    status: STATUS_BY_PROTO[step.status],
    startedAt: step.startedAt ? timestampDate(step.startedAt) : undefined,
    finishedAt: step.finishedAt ? timestampDate(step.finishedAt) : undefined,
    message: step.message,
    detail: step.detail,
  }
}

// taskFromProto converts one wire purser.job.v1.Task, including its Steps.
function taskFromProto(task: ProtoTask): Task {
  return {
    id: task.id,
    label: task.label,
    status: STATUS_BY_PROTO[task.status],
    startedAt: task.startedAt ? timestampDate(task.startedAt) : undefined,
    finishedAt: task.finishedAt ? timestampDate(task.finishedAt) : undefined,
    steps: task.steps.map(stepFromProto),
    progress: task.progress,
  }
}

// jobFromProto converts one wire purser.job.v1.Job into the plain Job app
// code is written against (web/src/types/index.ts) — mirrors
// settingFromProto's shape. The full Task/Step tree and params are
// carried through unconditionally: the Jobs tab (#606) table ignores
// them, and the job detail modal (#608) reads them.
export function jobFromProto(job: ProtoJob): Job {
  return {
    id: job.id,
    kind: job.kind,
    status: STATUS_BY_PROTO[job.status],
    createdAt: job.createdAt ? timestampDate(job.createdAt) : undefined,
    startedAt: job.startedAt ? timestampDate(job.startedAt) : undefined,
    finishedAt: job.finishedAt ? timestampDate(job.finishedAt) : undefined,
    progress: job.progress,
    tasks: job.tasks.map(taskFromProto),
    params: job.params,
  }
}
