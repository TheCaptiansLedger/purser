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
};
