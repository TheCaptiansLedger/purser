import { JobStatusBadge } from '../../components/JobStatusBadge'
import { useJobsList } from '../../hooks/useJobsList'
import { jobFromProto } from './jobFromProto'
import { formatJobProgress, formatJobTimestamp } from './jobFormat'

// JobsTab is #606's Jobs tab: a cursor-paginated (ListJobs, page_size/
// page_token — see docs/adr/0023-job-queue.md), scrollable table of every
// job the server has tracked. Row click / live detail (WatchJob) is
// #607/#608, not this pass — see docs/adr/0011-api-design.md.
export function JobsTab() {
  const jobsQuery = useJobsList()

  // Doherty threshold — see docs/design/ux-principles.md#feedback--system-status.
  // A local Connect round trip resolves well under 400ms; a loading
  // indicator here would read as slower, not more informative.
  if (jobsQuery.isPending) {
    return null
  }

  if (jobsQuery.isError) {
    return (
      <p className="text-status-failure text-body" role="alert">
        Couldn't load jobs ({jobsQuery.error.message}).
      </p>
    )
  }

  const jobs = jobsQuery.data.pages.flatMap(page => page.jobs).map(jobFromProto)

  if (jobs.length === 0) {
    return <p className="text-body text-text-secondary">No jobs have run yet.</p>
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="max-h-[32rem] overflow-y-auto rounded-lg border border-border">
        <table className="w-full text-left text-body">
          <thead className="sticky top-0 bg-surface-raised text-label text-text-secondary">
            <tr>
              <th className="px-3 py-2 font-medium">Kind</th>
              <th className="px-3 py-2 font-medium">Status</th>
              <th className="px-3 py-2 font-medium">Created</th>
              <th className="px-3 py-2 font-medium">Started</th>
              <th className="px-3 py-2 font-medium">Finished</th>
              <th className="px-3 py-2 font-medium">Progress</th>
            </tr>
          </thead>
          <tbody>
            {jobs.map(job => (
              <tr key={job.id} className="border-t border-border">
                <td className="px-3 py-2 text-text">{job.kind}</td>
                <td className="px-3 py-2">
                  <JobStatusBadge status={job.status} />
                </td>
                <td className="px-3 py-2 text-text-secondary">{formatJobTimestamp(job.createdAt)}</td>
                <td className="px-3 py-2 text-text-secondary">{formatJobTimestamp(job.startedAt)}</td>
                <td className="px-3 py-2 text-text-secondary">{formatJobTimestamp(job.finishedAt)}</td>
                <td className="px-3 py-2 text-text-secondary">{formatJobProgress(job.progress)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {jobsQuery.hasNextPage && (
        <div className="flex justify-center">
          <button
            type="button"
            onClick={() => void jobsQuery.fetchNextPage()}
            disabled={jobsQuery.isFetchingNextPage}
            className="h-9 px-4 rounded-lg bg-surface-raised text-text text-body font-medium hover:opacity-90 disabled:opacity-50"
          >
            {jobsQuery.isFetchingNextPage ? 'Loading…' : 'Load more'}
          </button>
        </div>
      )}
    </div>
  )
}
