import { useQuery } from '@connectrpc/connect-query'
import { listJobs } from '../gen/purser/job/v1/job-JobService_connectquery'

// useJobs wraps JobService.ListJobs (see proto/purser/job/v1/job.proto) —
// a small, side-effect-free call used here purely to prove the Connect-Web
// wiring works end-to-end. pageSize is kept small since the Welcome page
// only needs a count, not the full list — see
// docs/design/ux-principles.md#feedback--system-status.
export function useJobs() {
  return useQuery(listJobs, { pageSize: 1 })
}
