import { useInfiniteQuery } from '@connectrpc/connect-query'
import { listJobs } from '../gen/purser/job/v1/job-JobService_connectquery'

// JOBS_PAGE_SIZE is the Jobs tab's (#606) page size — independent of
// pkg/jobqueue/memory's own defaultPageSize (50), which only applies when
// a caller sends pageSize <= 0.
const JOBS_PAGE_SIZE = 25

// useJobsList wraps JobService.ListJobs (see proto/purser/job/v1/job.proto)
// as a cursor-paginated infinite query — the Jobs tab's scrollable table.
// Kept separate from useJobs (the Welcome page's fixed pageSize:1 count
// hook): same RPC, different shape/purpose, same one-hook-per-call-shape
// split useSettings.ts already established for its mutations.
export function useJobsList() {
  return useInfiniteQuery(
    listJobs,
    { pageSize: JOBS_PAGE_SIZE, pageToken: '' },
    {
      pageParamKey: 'pageToken',
      getNextPageParam: lastPage => (lastPage.nextPageToken !== '' ? lastPage.nextPageToken : undefined),
    },
  )
}
