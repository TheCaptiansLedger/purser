import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { create } from '@bufbuild/protobuf'
import { timestampFromDate } from '@bufbuild/protobuf/wkt'
import type { MessageInitShape } from '@bufbuild/protobuf'
import { useJobsList } from '../../hooks/useJobsList'
import { JobSchema, JobStatus } from '../../gen/purser/job/v1/job_pb'
import { JobsTab } from './JobsTab'

// Page-level test: composition only, per ADR 0004 — jobFromProto's own
// conversion is tested in jobFromProto.test.ts, JobStatusBadge's own
// color/label mapping in JobStatusBadge.test.tsx, and useJobsList's wire
// behavior in useJobsList.test.tsx. Here the hook is mocked directly so
// every state is reachable deterministically.
vi.mock('../../hooks/useJobsList')
const mockUseJobsList = vi.mocked(useJobsList)

function proto(overrides: MessageInitShape<typeof JobSchema>) {
  return create(JobSchema, {
    id: 'job-1',
    kind: 'scan',
    status: JobStatus.RUNNING,
    createdAt: timestampFromDate(new Date('2026-08-09T12:00:00Z')),
    progress: 0.5,
    ...overrides,
  })
}

describe('JobsTab', () => {
  it('renders nothing while pending', () => {
    mockUseJobsList.mockReturnValue({
      isPending: true,
      isError: false,
      data: undefined,
      error: null,
      hasNextPage: false,
      isFetchingNextPage: false,
      fetchNextPage: vi.fn(),
    } as unknown as ReturnType<typeof useJobsList>)

    const { container } = render(<JobsTab />)
    expect(container.textContent).toBe('')
  })

  it('shows an actionable error when ListJobs fails', () => {
    mockUseJobsList.mockReturnValue({
      isPending: false,
      isError: true,
      data: undefined,
      error: { message: 'unavailable' },
      hasNextPage: false,
      isFetchingNextPage: false,
      fetchNextPage: vi.fn(),
    } as unknown as ReturnType<typeof useJobsList>)

    render(<JobsTab />)
    expect(screen.getByRole('alert')).toHaveTextContent("Couldn't load jobs (unavailable)")
  })

  it('shows an empty state when there are no jobs', () => {
    mockUseJobsList.mockReturnValue({
      isPending: false,
      isError: false,
      data: { pages: [{ jobs: [], nextPageToken: '' }] },
      error: null,
      hasNextPage: false,
      isFetchingNextPage: false,
      fetchNextPage: vi.fn(),
    } as unknown as ReturnType<typeof useJobsList>)

    render(<JobsTab />)
    expect(screen.getByText('No jobs have run yet.')).toBeInTheDocument()
  })

  it('renders one row per job across pages, with kind/status/progress', () => {
    mockUseJobsList.mockReturnValue({
      isPending: false,
      isError: false,
      data: {
        pages: [
          { jobs: [proto({ id: 'job-1', kind: 'scan', status: JobStatus.RUNNING, progress: 0.5 })], nextPageToken: 'job-1' },
          { jobs: [proto({ id: 'job-2', kind: 'identify', status: JobStatus.SUCCEEDED, progress: 1 })], nextPageToken: '' },
        ],
      },
      error: null,
      hasNextPage: false,
      isFetchingNextPage: false,
      fetchNextPage: vi.fn(),
    } as unknown as ReturnType<typeof useJobsList>)

    render(<JobsTab />)

    expect(screen.getByText('scan')).toBeInTheDocument()
    expect(screen.getByText('identify')).toBeInTheDocument()
    expect(screen.getByText('Running')).toBeInTheDocument()
    expect(screen.getByText('Succeeded')).toBeInTheDocument()
    expect(screen.getByText('50%')).toBeInTheDocument()
    expect(screen.getByText('100%')).toBeInTheDocument()
  })

  it('calls fetchNextPage when Load more is clicked, and hides it once exhausted', () => {
    const fetchNextPage = vi.fn()
    mockUseJobsList.mockReturnValue({
      isPending: false,
      isError: false,
      data: { pages: [{ jobs: [proto({})], nextPageToken: 'job-1' }] },
      error: null,
      hasNextPage: true,
      isFetchingNextPage: false,
      fetchNextPage,
    } as unknown as ReturnType<typeof useJobsList>)

    const { rerender } = render(<JobsTab />)
    fireEvent.click(screen.getByRole('button', { name: 'Load more' }))
    expect(fetchNextPage).toHaveBeenCalled()

    mockUseJobsList.mockReturnValue({
      isPending: false,
      isError: false,
      data: { pages: [{ jobs: [proto({})], nextPageToken: '' }] },
      error: null,
      hasNextPage: false,
      isFetchingNextPage: false,
      fetchNextPage,
    } as unknown as ReturnType<typeof useJobsList>)

    rerender(<JobsTab />)
    expect(screen.queryByRole('button', { name: 'Load more' })).not.toBeInTheDocument()
  })
})
