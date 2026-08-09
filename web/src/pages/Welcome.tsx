import { useJobs } from '../hooks/useJobs'

// Welcome is the app's only route today — no module screens exist yet
// (see docs/design/frontend-stack.md). It exists to prove the framework
// is wired end-to-end: styling, routing, and one real Connect RPC
// round-trip (JobService.ListJobs).
//
// Status line follows docs/design/ux-principles.md#feedback--system-status:
// nothing renders while pending (the common case resolves well under the
// Doherty threshold on a local server, so a spinner would just be noise),
// an error is actionable rather than a raw dump, and success is stated
// honestly — pageSize is small, so this never claims a total job count
// it can't back up.
export function Welcome() {
  const { data, isPending, isError, error } = useJobs()

  return (
    <div className="px-6 py-10 md:px-8">
      <h1 className="text-headline text-text">Welcome to Purser</h1>
      <p className="text-body text-text-secondary mt-2">
        The framework is wired up — routing, styling, and the Connect RPC client are all live.
      </p>

      <div className="mt-6 text-body-lg text-text" role="status" aria-live="polite">
        {isPending && null}
        {isError && (
          <p className="text-status-failure">
            Couldn't reach the Purser API ({error.message}). Confirm the server is running (
            <code className="text-label bg-surface rounded-sm px-1 py-0.5">make run</code>) and reachable.
          </p>
        )}
        {data && (
          <p className="text-status-success">
            {jobCountMessage(data.jobs.length, data.nextPageToken !== '')}
          </p>
        )}
      </div>
    </div>
  )
}

// jobCountMessage is deliberately honest about what a pageSize-limited
// response can and can't claim — see the Welcome doc comment above.
function jobCountMessage(returned: number, hasMore: boolean): string {
  if (returned === 0) {
    return 'Connected — no jobs currently running.'
  }
  if (hasMore) {
    return `Connected — ${returned}+ job(s) in the queue.`
  }
  return `Connected — ${returned} job(s) in the queue.`
}
