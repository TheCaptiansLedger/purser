import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { create } from '@bufbuild/protobuf'
import { timestampFromDate } from '@bufbuild/protobuf/wkt'
import { ConnectError } from '@connectrpc/connect'
import { useWatchJob } from '../../hooks/useWatchJob'
import { JobSchema, JobStatus } from '../../gen/purser/job/v1/job_pb'
import { JobDetailModal } from './JobDetailModal'

// Page-level test per ADR 0004: useWatchJob's own streaming behavior is
// tested in useWatchJob.test.tsx, jobFromProto's conversion in
// jobFromProto.test.ts, and Modal's dialog chrome in Modal.test.tsx — here
// the hook is mocked directly so every state (loading/error/populated) is
// reachable deterministically.
vi.mock('../../hooks/useWatchJob')
const mockUseWatchJob = vi.mocked(useWatchJob)

describe('JobDetailModal', () => {
  it('shows a loading state before the first WatchJob event arrives', () => {
    mockUseWatchJob.mockReturnValue({ job: undefined, event: undefined, error: undefined, done: false })

    render(<JobDetailModal jobId="job-1" onClose={vi.fn()} />)

    expect(screen.getByRole('dialog', { name: 'Job detail' })).toBeInTheDocument()
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })

  it('shows an actionable error when the stream fails', () => {
    mockUseWatchJob.mockReturnValue({
      job: undefined,
      event: undefined,
      error: new ConnectError('unavailable'),
      done: false,
    })

    render(<JobDetailModal jobId="job-1" onClose={vi.fn()} />)

    expect(screen.getByRole('alert')).toHaveTextContent("Couldn't load job")
  })

  it('renders the full Task/Step tree, params, and timestamps once the job arrives', () => {
    const job = create(JobSchema, {
      id: 'job-1',
      kind: 'scan',
      status: JobStatus.RUNNING,
      createdAt: timestampFromDate(new Date('2026-08-09T12:00:00Z')),
      startedAt: timestampFromDate(new Date('2026-08-09T12:00:01Z')),
      progress: 0.5,
      params: { fail_at_step: 'a' },
      tasks: [
        {
          id: 'task-1',
          label: '03 - Bella Donna.flac',
          status: JobStatus.RUNNING,
          progress: 0.5,
          startedAt: timestampFromDate(new Date('2026-08-09T12:00:02Z')),
          steps: [
            {
              id: 'step-1',
              name: 'compute hashes',
              status: JobStatus.SUCCEEDED,
              startedAt: timestampFromDate(new Date('2026-08-09T12:00:02Z')),
              finishedAt: timestampFromDate(new Date('2026-08-09T12:00:03Z')),
              message: 'sha256 computed',
              detail: { sha256: 'abc123' },
            },
          ],
        },
      ],
    })
    mockUseWatchJob.mockReturnValue({ job, event: undefined, error: undefined, done: false })

    render(<JobDetailModal jobId="job-1" onClose={vi.fn()} />)

    expect(screen.getByRole('dialog', { name: 'scan — job-1' })).toBeInTheDocument()
    // "Running"/"50%" appear twice — once for the Job, once for its Task.
    expect(screen.getAllByText('Running')).toHaveLength(2)
    expect(screen.getAllByText('50%')).toHaveLength(2)
    expect(screen.getByText('fail_at_step')).toBeInTheDocument()
    expect(screen.getByText('a')).toBeInTheDocument()
    expect(screen.getByText('03 - Bella Donna.flac')).toBeInTheDocument()
    expect(screen.getByText('compute hashes')).toBeInTheDocument()
    expect(screen.getByText('Succeeded')).toBeInTheDocument()
    expect(screen.getByText('sha256 computed')).toBeInTheDocument()
    expect(screen.getByText('sha256')).toBeInTheDocument()
    expect(screen.getByText('abc123')).toBeInTheDocument()
  })

  it('shows a no-tasks placeholder and omits the Params section when both are empty', () => {
    const job = create(JobSchema, { id: 'job-2', kind: 'scan', status: JobStatus.PENDING })
    mockUseWatchJob.mockReturnValue({ job, event: undefined, error: undefined, done: false })

    render(<JobDetailModal jobId="job-2" onClose={vi.fn()} />)

    expect(screen.getByText('No tasks yet.')).toBeInTheDocument()
    expect(screen.queryByText('Params')).not.toBeInTheDocument()
  })

  it('re-renders with the latest snapshot as useWatchJob delivers new events', () => {
    const running = create(JobSchema, { id: 'job-3', kind: 'scan', status: JobStatus.RUNNING, progress: 0.2 })
    mockUseWatchJob.mockReturnValue({ job: running, event: undefined, error: undefined, done: false })
    const { rerender } = render(<JobDetailModal jobId="job-3" onClose={vi.fn()} />)
    expect(screen.getByText('Running')).toBeInTheDocument()

    const succeeded = create(JobSchema, { id: 'job-3', kind: 'scan', status: JobStatus.SUCCEEDED, progress: 1 })
    mockUseWatchJob.mockReturnValue({ job: succeeded, event: undefined, error: undefined, done: true })
    rerender(<JobDetailModal jobId="job-3" onClose={vi.fn()} />)

    expect(screen.getByText('Succeeded')).toBeInTheDocument()
    expect(screen.getByText('100%')).toBeInTheDocument()
  })
})
