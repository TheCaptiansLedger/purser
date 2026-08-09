import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { useJobs } from '../hooks/useJobs'
import { Welcome } from './Welcome'

// Page-level test: composition only, per ADR 0004 — useJobs itself is
// tested against a mocked transport in hooks/useJobs.test.tsx. Here the
// hook is mocked directly so every loading/success/error state is
// reachable deterministically without touching Connect at all.
vi.mock('../hooks/useJobs')
const mockUseJobs = vi.mocked(useJobs)

describe('Welcome', () => {
  it('renders nothing in the status region while pending', () => {
    mockUseJobs.mockReturnValue({
      isPending: true,
      isError: false,
      isSuccess: false,
      data: undefined,
      error: null,
    } as unknown as ReturnType<typeof useJobs>)
    render(<Welcome />)
    expect(screen.getByRole('heading', { name: 'Welcome to Purser' })).toBeInTheDocument()
    expect(screen.getByRole('status')).toBeEmptyDOMElement()
  })

  it('shows an actionable message on error', () => {
    mockUseJobs.mockReturnValue({
      isPending: false,
      isError: true,
      isSuccess: false,
      data: undefined,
      error: { message: 'unavailable' },
    } as unknown as ReturnType<typeof useJobs>)
    render(<Welcome />)
    expect(screen.getByText(/Couldn't reach the Purser API \(unavailable\)/)).toBeInTheDocument()
  })

  it('states an honest job count on success with no more pages', () => {
    mockUseJobs.mockReturnValue({
      isPending: false,
      isError: false,
      isSuccess: true,
      data: { jobs: [{ id: 'job-1' }], nextPageToken: '' },
      error: null,
    } as unknown as ReturnType<typeof useJobs>)
    render(<Welcome />)
    expect(screen.getByText('Connected — 1 job(s) in the queue.')).toBeInTheDocument()
  })

  it('does not overclaim a total when more pages exist', () => {
    mockUseJobs.mockReturnValue({
      isPending: false,
      isError: false,
      isSuccess: true,
      data: { jobs: [{ id: 'job-1' }], nextPageToken: 'more' },
      error: null,
    } as unknown as ReturnType<typeof useJobs>)
    render(<Welcome />)
    expect(screen.getByText('Connected — 1+ job(s) in the queue.')).toBeInTheDocument()
  })

  it('reports zero jobs plainly', () => {
    mockUseJobs.mockReturnValue({
      isPending: false,
      isError: false,
      isSuccess: true,
      data: { jobs: [], nextPageToken: '' },
      error: null,
    } as unknown as ReturnType<typeof useJobs>)
    render(<Welcome />)
    expect(screen.getByText('Connected — no jobs currently running.')).toBeInTheDocument()
  })
})
