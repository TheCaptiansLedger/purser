import { timestampDate } from '@bufbuild/protobuf/wkt'
import { JobStatus as ProtoJobStatus } from '../../gen/purser/job/v1/job_pb'
import type { Job as ProtoJob } from '../../gen/purser/job/v1/job_pb'
import type { Job, JobStatus } from '../../types'

const STATUS_BY_PROTO: Record<ProtoJobStatus, JobStatus> = {
  [ProtoJobStatus.UNSPECIFIED]: 'unspecified',
  [ProtoJobStatus.PENDING]: 'pending',
  [ProtoJobStatus.RUNNING]: 'running',
  [ProtoJobStatus.SUCCEEDED]: 'succeeded',
  [ProtoJobStatus.FAILED]: 'failed',
  [ProtoJobStatus.PARTIAL]: 'partial',
}

// jobFromProto converts one wire purser.job.v1.Job into the plain Job app
// code is written against (web/src/types/index.ts) — mirrors
// settingFromProto's shape. Task/Step trees aren't carried through; the
// Jobs tab (#606) only reads the fields above until the detail modal
// (#608).
export function jobFromProto(job: ProtoJob): Job {
  return {
    id: job.id,
    kind: job.kind,
    status: STATUS_BY_PROTO[job.status],
    createdAt: job.createdAt ? timestampDate(job.createdAt) : undefined,
    startedAt: job.startedAt ? timestampDate(job.startedAt) : undefined,
    finishedAt: job.finishedAt ? timestampDate(job.finishedAt) : undefined,
    progress: job.progress,
  }
}
