// k6 gRPC suite for ScanService. See test/k6/grpc/job_test.js for the
// polling-until-terminal pattern this follows. This is the walking
// skeleton for the Common Scan Pipeline (#486) — every later sub-issue
// widens this same file. See docs/adr/0024-pipeline-core.md.
//
// PURSER_SCAN_FIXTURE_ROOT must point at a real directory the *server*
// process can see, containing at least one file >= 64 KiB (OSHash's
// minimum chunk size — see pkg/filehash). Default '/media/content/scan'
// matches ops/compose.yml's ../test-data bind mount; make k6-ci overrides
// it to the hermetic .cidata/scan fixture it creates itself.
import grpc from 'k6/net/grpc';
import { check, sleep } from 'k6';
import { options } from '../lib/options.js';
export { options };

const ADDR = __ENV.PURSER_GRPC_ADDR || 'localhost:7474';
const FIXTURE_ROOT = __ENV.PURSER_SCAN_FIXTURE_ROOT || '/media/content/scan';
const FIXTURE_FILE_COUNT = parseInt(__ENV.PURSER_SCAN_FIXTURE_FILE_COUNT || '3', 10);

const client = new grpc.Client();
client.load(['../../../proto'], 'purser/pipeline/v1/scan.proto', 'purser/job/v1/job.proto');

function invoke(method, request) {
  const res = client.invoke(method, request);
  console.log(JSON.stringify({ method: method, request: request, response: res.message }, null, 2));
  return res;
}

const terminal = ['JOB_STATUS_SUCCEEDED', 'JOB_STATUS_FAILED', 'JOB_STATUS_PARTIAL'];

function waitForTerminalJob(jobId) {
  let job = null;
  for (let i = 0; i < 100; i++) {
    const res = invoke('purser.job.v1.JobService/GetJob', { id: jobId });
    check(res, { 'GetJob status is OK': (r) => r && r.status === grpc.StatusOK });
    job = res.message.job;
    if (terminal.indexOf(job.status) !== -1) {
      break;
    }
    sleep(0.1);
  }
  return job;
}

export default () => {
  client.connect(ADDR, { plaintext: true });

  // Once with both hashing toggles at whatever the server was started
  // with (default: both off) — proves the walking skeleton end-to-end:
  // one Task per discovered file, a hash/queue Step sequence, OSHash/SHA1
  // always populated.
  let res = invoke('purser.pipeline.v1.ScanService/TriggerScan', { root: FIXTURE_ROOT });
  check(res, {
    'TriggerScan status is OK': (r) => r && r.status === grpc.StatusOK,
    'TriggerScan returns a job id': (r) => r && r.message && !!r.message.jobId,
  });
  const jobId = res.message.jobId;

  const job = waitForTerminalJob(jobId);
  check(job, {
    'scan job reached a terminal status': (j) => j && terminal.indexOf(j.status) !== -1,
    'scan job succeeded': (j) => j && j.status === 'JOB_STATUS_SUCCEEDED',
    [`scan job has ${FIXTURE_FILE_COUNT} tasks (one per fixture file)`]: (j) => j && j.tasks && j.tasks.length === FIXTURE_FILE_COUNT,
    'every task has a hash then queue step': (j) =>
      j.tasks.every((t) => t.steps && t.steps.length === 2 && t.steps[0].name === 'hash' && t.steps[1].name === 'queue'),
    'every hash step succeeded with oshash/sha1 detail': (j) =>
      j.tasks.every((t) => {
        const hash = t.steps[0];
        return hash.status === 'JOB_STATUS_SUCCEEDED' && hash.detail && !!hash.detail.oshash && !!hash.detail.sha1;
      }),
    'every queue step succeeded with an unmatched_file.id detail': (j) =>
      j.tasks.every((t) => {
        const queue = t.steps[1];
        return queue.status === 'JOB_STATUS_SUCCEEDED' && queue.detail && !!queue.detail['unmatched_file.id'];
      }),
  });

  client.close();
};
