// k6 HTTP/JSON suite for JobService. See test/k6/http/music_release_test.js
// for the pattern this follows. This file is extended by every later
// sub-issue in the Job Queue epic, not replaced. See
// docs/adr/0023-job-queue.md.
import http from 'k6/http';
import { check, sleep } from 'k6';
import { options } from '../lib/options.js';
export { options };

const BASE_URL = __ENV.PURSER_HTTP_URL || 'http://localhost:7474';
const SERVICE = `${BASE_URL}/purser.job.v1.JobService`;
const HEADERS = { headers: { 'Content-Type': 'application/json' } };

function invoke(url, body, headers) {
  const res = http.post(url, body, headers);
  console.log(JSON.stringify({ method: url, request: JSON.parse(body), response: res.json() }, null, 2));
  return res;
}

export default () => {
  const taskLabels = ['track one', 'track two', 'track three'];
  let res = invoke(`${SERVICE}/TriggerJob`, JSON.stringify({ kind: 'diagnostic', taskLabels: taskLabels }), HEADERS);
  check(res, {
    'TriggerJob status is 200': (r) => r.status === 200,
    'TriggerJob returns a job id': (r) => !!r.json('jobId'),
  });
  const jobId = res.json('jobId');

  // Poll GetJob until the job reaches a terminal status — the diagnostic
  // executor always succeeds, but it runs asynchronously (real, brief
  // per-step delays), so this proves pending -> running -> succeeded, not
  // just an instantaneous response.
  let job = null;
  const terminal = ['JOB_STATUS_SUCCEEDED', 'JOB_STATUS_FAILED', 'JOB_STATUS_PARTIAL'];
  for (let i = 0; i < 50; i++) {
    res = invoke(`${SERVICE}/GetJob`, JSON.stringify({ id: jobId }), HEADERS);
    check(res, { 'GetJob status is 200': (r) => r.status === 200 });
    job = res.json('job');
    if (terminal.indexOf(job.status) !== -1) {
      break;
    }
    sleep(0.1);
  }

  check(job, {
    'job reached a terminal status': (j) => terminal.indexOf(j.status) !== -1,
    'job succeeded': (j) => j.status === 'JOB_STATUS_SUCCEEDED',
    'job has 3 tasks': (j) => j.tasks && j.tasks.length === 3,
    'every task succeeded': (j) => j.tasks.every((t) => t.status === 'JOB_STATUS_SUCCEEDED'),
    'every task has 3 steps': (j) => j.tasks.every((t) => t.steps && t.steps.length === 3),
    'every step succeeded with elapsed_ms detail': (j) =>
      j.tasks.every((t) => t.steps.every((s) => s.status === 'JOB_STATUS_SUCCEEDED' && s.detail && !!s.detail.elapsed_ms)),
    'job progress is 1': (j) => j.progress === 1,
  });

  // Failure/partial-status handling (#482): a diagnostic job with exactly
  // one task's step configured to fail must aggregate to JOB_STATUS_PARTIAL
  // per docs/adr/0023-job-queue.md's rule — failed only if every task
  // failed, partial if at least one succeeded and at least one failed.
  res = invoke(
    `${SERVICE}/TriggerJob`,
    JSON.stringify({ kind: 'diagnostic', taskLabels: taskLabels, params: { 'fail_at_step:track two': '1' } }),
    HEADERS
  );
  check(res, {
    'TriggerJob (partial case) status is 200': (r) => r.status === 200,
    'TriggerJob (partial case) returns a job id': (r) => !!r.json('jobId'),
  });
  const partialJobId = res.json('jobId');

  let partialJob = null;
  for (let i = 0; i < 50; i++) {
    res = invoke(`${SERVICE}/GetJob`, JSON.stringify({ id: partialJobId }), HEADERS);
    check(res, { 'GetJob (partial case) status is 200': (r) => r.status === 200 });
    partialJob = res.json('job');
    if (terminal.indexOf(partialJob.status) !== -1) {
      break;
    }
    sleep(0.1);
  }

  check(partialJob, {
    'partial job status is JOB_STATUS_PARTIAL': (j) => j.status === 'JOB_STATUS_PARTIAL',
    'partial job has 3 tasks': (j) => j.tasks && j.tasks.length === 3,
    'exactly one task failed': (j) => j.tasks.filter((t) => t.status === 'JOB_STATUS_FAILED').length === 1,
    'the other two tasks succeeded': (j) => j.tasks.filter((t) => t.status === 'JOB_STATUS_SUCCEEDED').length === 2,
    'the failed task has a step with populated detail': (j) => {
      const failedTask = j.tasks.find((t) => t.status === 'JOB_STATUS_FAILED');
      const failedStep = failedTask && failedTask.steps.find((s) => s.status === 'JOB_STATUS_FAILED');
      return !!failedStep && !!failedStep.message && !!failedStep.detail && !!failedStep.detail.elapsed_ms;
    },
  });

  // Every task's steps configured to fail must aggregate to
  // JOB_STATUS_FAILED, the other half of the same rule.
  const failParams = {};
  taskLabels.forEach((label) => {
    failParams[`fail_at_step:${label}`] = '0';
  });
  res = invoke(`${SERVICE}/TriggerJob`, JSON.stringify({ kind: 'diagnostic', taskLabels: taskLabels, params: failParams }), HEADERS);
  check(res, {
    'TriggerJob (failed case) status is 200': (r) => r.status === 200,
    'TriggerJob (failed case) returns a job id': (r) => !!r.json('jobId'),
  });
  const failedJobId = res.json('jobId');

  let failedJob = null;
  for (let i = 0; i < 50; i++) {
    res = invoke(`${SERVICE}/GetJob`, JSON.stringify({ id: failedJobId }), HEADERS);
    check(res, { 'GetJob (failed case) status is 200': (r) => r.status === 200 });
    failedJob = res.json('job');
    if (terminal.indexOf(failedJob.status) !== -1) {
      break;
    }
    sleep(0.1);
  }

  check(failedJob, {
    'failed job status is JOB_STATUS_FAILED': (j) => j.status === 'JOB_STATUS_FAILED',
    'every task failed': (j) => j.tasks.every((t) => t.status === 'JOB_STATUS_FAILED'),
  });
};
