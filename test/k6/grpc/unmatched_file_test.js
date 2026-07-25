// k6 gRPC suite for UnmatchedFileService. See test/k6/grpc/scan_test.js for
// the fixture/polling pattern this reuses to seed real UnmatchedFile rows —
// TriggerScan's queue step details (#487) carry the ids this suite reads.
// UnmatchedFile is Datastore-backed (durable, docs/adr/0024-pipeline-core.md),
// so the server-side store accumulates across k6 runs the same way Job's
// in-memory store does (see test/k6/grpc/job_test.js) — this suite proves
// the seeded ids show up correctly rather than asserting an exact total
// record count.
import grpc from 'k6/net/grpc';
import { check, sleep } from 'k6';
import { options } from '../lib/options.js';
export { options };

const ADDR = __ENV.PURSER_GRPC_ADDR || 'localhost:7474';
const FIXTURE_ROOT = __ENV.PURSER_SCAN_FIXTURE_ROOT || '/media/content/scan';
const FIXTURE_FILE_COUNT = parseInt(__ENV.PURSER_SCAN_FIXTURE_FILE_COUNT || '3', 10);

const client = new grpc.Client();
client.load(['../../../proto'], 'purser/pipeline/v1/scan.proto', 'purser/pipeline/v1/unmatched_file.proto', 'purser/job/v1/job.proto');

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

function triggerScanAndCollectIds(root, wantCount) {
  const res = invoke('purser.pipeline.v1.ScanService/TriggerScan', { root: root });
  check(res, {
    'TriggerScan status is OK': (r) => r && r.status === grpc.StatusOK,
    'TriggerScan returns a job id': (r) => r && r.message && !!r.message.jobId,
  });
  const job = waitForTerminalJob(res.message.jobId);
  check(job, {
    'scan job succeeded': (j) => j && j.status === 'JOB_STATUS_SUCCEEDED',
    [`scan job has ${wantCount} tasks`]: (j) => j && j.tasks && j.tasks.length === wantCount,
  });
  return job.tasks.map((t) => t.steps[1].detail['unmatched_file.id']);
}

export default () => {
  client.connect(ADDR, { plaintext: true });

  const seededIds = triggerScanAndCollectIds(FIXTURE_ROOT, FIXTURE_FILE_COUNT);

  // GetUnmatchedFile on one of the seeded ids — the full hash set (OSHash/
  // SHA1 always populated; MD5/SHA512 depend on the server's hashing
  // toggles, so only presence of the always-on hashes is asserted here).
  let res = invoke('purser.pipeline.v1.UnmatchedFileService/GetUnmatchedFile', { id: seededIds[0] });
  check(res, {
    'GetUnmatchedFile status is OK': (r) => r && r.status === grpc.StatusOK,
    'GetUnmatchedFile returns the requested id': (r) => r.message.unmatchedFile.id === seededIds[0],
    'GetUnmatchedFile returns osHash/sha1': (r) => !!r.message.unmatchedFile.osHash && !!r.message.unmatchedFile.sha1,
    'GetUnmatchedFile status is pending': (r) => r.message.unmatchedFile.status === 'UNMATCHED_FILE_STATUS_PENDING',
  });

  // ListUnmatchedFiles with no filter must include every seeded id, each
  // still pending.
  res = invoke('purser.pipeline.v1.UnmatchedFileService/ListUnmatchedFiles', { pageSize: 50 });
  check(res, {
    'ListUnmatchedFiles (no filter) status is OK': (r) => r && r.status === grpc.StatusOK,
    'ListUnmatchedFiles (no filter) includes every seeded id': (r) => {
      const gotIds = (r.message.unmatchedFiles || []).map((u) => u.id);
      return seededIds.every((id) => gotIds.indexOf(id) !== -1);
    },
    'ListUnmatchedFiles (no filter) seeded records are pending': (r) => {
      const byId = {};
      (r.message.unmatchedFiles || []).forEach((u) => (byId[u.id] = u));
      return seededIds.every((id) => byId[id] && byId[id].status === 'UNMATCHED_FILE_STATUS_PENDING');
    },
  });

  // ListUnmatchedFiles filtered by status=pending must include every
  // seeded id, and every returned record must actually be pending.
  res = invoke('purser.pipeline.v1.UnmatchedFileService/ListUnmatchedFiles', {
    pageSize: 50,
    status: 'UNMATCHED_FILE_STATUS_PENDING',
  });
  check(res, {
    'ListUnmatchedFiles (status=pending) status is OK': (r) => r && r.status === grpc.StatusOK,
    'ListUnmatchedFiles (status=pending) includes every seeded id': (r) => {
      const gotIds = (r.message.unmatchedFiles || []).map((u) => u.id);
      return seededIds.every((id) => gotIds.indexOf(id) !== -1);
    },
    'ListUnmatchedFiles (status=pending) every result is pending': (r) =>
      (r.message.unmatchedFiles || []).every((u) => u.status === 'UNMATCHED_FILE_STATUS_PENDING'),
  });

  // Pagination: page_size smaller than the total record count must page
  // through with no id repeated and no gap across the seeded ids. The
  // store is long-lived across k6 runs (like Job's), so this walks every
  // page rather than assuming the seeded records are the only ones.
  res = invoke('purser.pipeline.v1.UnmatchedFileService/ListUnmatchedFiles', { pageSize: 2 });
  check(res, {
    'ListUnmatchedFiles (page 1) status is OK': (r) => r && r.status === grpc.StatusOK,
    'ListUnmatchedFiles (page 1) returns 2 records': (r) => (r.message.unmatchedFiles || []).length === 2,
    'ListUnmatchedFiles (page 1) returns a next_page_token': (r) => !!r.message.nextPageToken,
  });
  let allIds = (res.message.unmatchedFiles || []).map((u) => u.id);
  let token = res.message.nextPageToken;
  while (token) {
    res = invoke('purser.pipeline.v1.UnmatchedFileService/ListUnmatchedFiles', { pageSize: 2, pageToken: token });
    check(res, { 'ListUnmatchedFiles (paging through) status is OK': (r) => r && r.status === grpc.StatusOK });
    allIds = allIds.concat((res.message.unmatchedFiles || []).map((u) => u.id));
    token = res.message.nextPageToken;
  }
  check(
    { allIds: allIds },
    {
      'ListUnmatchedFiles paginated with no id repeated across pages': (v) => new Set(v.allIds).size === v.allIds.length,
      'ListUnmatchedFiles paginated through every seeded id, no gap': (v) => seededIds.every((id) => v.allIds.indexOf(id) !== -1),
    }
  );

  client.close();
};
