import { createRouterTransport } from '@connectrpc/connect'
import { render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { JobService } from './gen/purser/job/v1/job_pb'

// A true end-to-end composition test: the real router, the real layout,
// the real Welcome page, and the real Connect-Query wiring — only the
// transport is swapped for an in-memory one (ADR 0004: never a live
// backend). This is what proves the framework is actually wired
// together, not just that each piece works in isolation.
vi.mock('./api/transport', () => ({
  transport: createRouterTransport(router => {
    router.service(JobService, {
      listJobs: () => ({ jobs: [], nextPageToken: '' }),
    })
  }),
}))

describe('App', () => {
  it('renders the welcome page through the full provider/router stack', async () => {
    const { default: App } = await import('./App')
    render(<App />)

    expect(screen.getByRole('heading', { name: 'Welcome to Purser' })).toBeInTheDocument()
    await waitFor(() => expect(screen.getByText('Connected — no jobs currently running.')).toBeInTheDocument())
  })
})
