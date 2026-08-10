import { describe, expect, it } from 'vitest'
import { create } from '@bufbuild/protobuf'
import { timestampFromDate } from '@bufbuild/protobuf/wkt'
import { JobSchema, JobStatus } from '../../gen/purser/job/v1/job_pb'
import { jobFromProto } from './jobFromProto'

describe('jobFromProto', () => {
  it('converts a running job with a startedAt but no finishedAt', () => {
    const createdAt = new Date('2026-08-09T12:00:00Z')
    const startedAt = new Date('2026-08-09T12:00:05Z')
    const proto = create(JobSchema, {
      id: 'job-1',
      kind: 'scan',
      status: JobStatus.RUNNING,
      createdAt: timestampFromDate(createdAt),
      startedAt: timestampFromDate(startedAt),
      progress: 0.4,
    })

    expect(jobFromProto(proto)).toEqual({
      id: 'job-1',
      kind: 'scan',
      status: 'running',
      createdAt,
      startedAt,
      finishedAt: undefined,
      progress: 0.4,
      tasks: [],
      params: {},
    })
  })

  it('converts a finished, partial job with all three timestamps set', () => {
    const createdAt = new Date('2026-08-09T12:00:00Z')
    const startedAt = new Date('2026-08-09T12:00:05Z')
    const finishedAt = new Date('2026-08-09T12:01:00Z')
    const proto = create(JobSchema, {
      id: 'job-2',
      kind: 'identify',
      status: JobStatus.PARTIAL,
      createdAt: timestampFromDate(createdAt),
      startedAt: timestampFromDate(startedAt),
      finishedAt: timestampFromDate(finishedAt),
      progress: 1,
    })

    expect(jobFromProto(proto)).toEqual({
      id: 'job-2',
      kind: 'identify',
      status: 'partial',
      createdAt,
      startedAt,
      finishedAt,
      progress: 1,
      tasks: [],
      params: {},
    })
  })

  it('carries the full Task/Step tree and params through, for the job detail modal (#608)', () => {
    const stepStartedAt = new Date('2026-08-09T12:00:05Z')
    const stepFinishedAt = new Date('2026-08-09T12:00:10Z')
    const proto = create(JobSchema, {
      id: 'job-3',
      kind: 'diagnostic',
      status: JobStatus.SUCCEEDED,
      progress: 1,
      params: { 'fail_at_step:a': '1' },
      tasks: [
        {
          id: 'task-1',
          label: '03 - Bella Donna.flac',
          status: JobStatus.SUCCEEDED,
          progress: 1,
          steps: [
            {
              id: 'step-1',
              name: 'compute hashes',
              status: JobStatus.SUCCEEDED,
              startedAt: timestampFromDate(stepStartedAt),
              finishedAt: timestampFromDate(stepFinishedAt),
              message: 'done',
              detail: { sha256: 'abc123' },
            },
          ],
        },
      ],
    })

    const got = jobFromProto(proto)

    expect(got.params).toEqual({ 'fail_at_step:a': '1' })
    expect(got.tasks).toEqual([
      {
        id: 'task-1',
        label: '03 - Bella Donna.flac',
        status: 'succeeded',
        startedAt: undefined,
        finishedAt: undefined,
        progress: 1,
        steps: [
          {
            id: 'step-1',
            name: 'compute hashes',
            status: 'succeeded',
            startedAt: stepStartedAt,
            finishedAt: stepFinishedAt,
            message: 'done',
            detail: { sha256: 'abc123' },
          },
        ],
      },
    ])
  })
})
