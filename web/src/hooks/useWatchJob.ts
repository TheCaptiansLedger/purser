import { ConnectError, createClient } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useEffect, useMemo, useState } from 'react'
import type { Job, JobEvent } from '../gen/purser/job/v1/job_pb'
import { JobService } from '../gen/purser/job/v1/job_pb'

export interface WatchJobState {
  // job is the latest full Job snapshot (tasks/steps included) carried by
  // the most recent JobEvent — the same shape GetJobResponse.job carries,
  // per docs/adr/0023-job-queue.md. undefined until the first event
  // arrives, and reset to undefined whenever jobId changes.
  job: Job | undefined
  // event is the raw latest JobEvent, for callers that want to know
  // exactly what just changed (kind/taskId/stepId) rather than only the
  // new snapshot.
  event: JobEvent | undefined
  // error is set if the stream ends abnormally (network failure, server
  // error). Cleared on every jobId change.
  error: ConnectError | undefined
  // done is true once the stream closes normally — the server ends
  // WatchJob once the job reaches a terminal status.
  done: boolean
}

// useWatchJob subscribes to JobService.WatchJob (see
// proto/purser/job/v1/job.proto) for one job's live task/step updates.
// connect-query has no generated hook for streaming RPCs, so this calls the
// raw generated JobServiceClient directly via createClient, using the same
// transport (from TransportProvider, mounted in App.tsx) every connect-query
// hook already shares — see docs/adr/0023-job-queue.md.
//
// This ports the poll+stream pattern proven in cmd/purser/jobs_model.go:
// subscribe on selection, tear down and resubscribe on selection change,
// tear down on unmount. There, a manually-incremented streamGen guards
// stale async messages from a superseded stream because Bubble Tea has no
// effect-cleanup primitive; here, a per-effect `cancelled` flag plus
// AbortController do the same job — React already tears down and re-runs
// the effect on every jobId change and guarantees the cleanup function
// runs first, so no counter is needed.
export function useWatchJob(jobId: string | undefined): WatchJobState {
  const transport = useTransport()
  const client = useMemo(() => createClient(JobService, transport), [transport])

  const [state, setState] = useState<WatchJobState>({
    job: undefined,
    event: undefined,
    error: undefined,
    done: false,
  })

  useEffect(() => {
    setState({ job: undefined, event: undefined, error: undefined, done: false })

    if (!jobId) {
      return
    }

    let cancelled = false
    const controller = new AbortController()

    async function run() {
      try {
        for await (const event of client.watchJob({ jobId }, { signal: controller.signal })) {
          if (cancelled) {
            return
          }
          setState(prev => ({ ...prev, event, job: event.job ?? prev.job }))
        }
        if (!cancelled) {
          setState(prev => ({ ...prev, done: true }))
        }
      } catch (err) {
        if (!cancelled) {
          setState(prev => ({ ...prev, error: ConnectError.from(err) }))
        }
      }
    }

    void run()

    return () => {
      cancelled = true
      controller.abort()
    }
  }, [client, jobId])

  return state
}
