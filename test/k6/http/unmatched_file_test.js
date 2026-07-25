// k6 HTTP/JSON suite for UnmatchedFileService. See test/k6/http/scan_test.js
// for the fixture/polling pattern this reuses to seed real UnmatchedFile
// rows — TriggerScan's check_known/queue step details (#487, #489) carry
// the ids this suite reads. UnmatchedFile is Datastore-backed (durable,
// docs/adr/0024-pipeline-core.md), so the server-side store accumulates
// across k6 runs the same way Job's in-memory store does (see
// test/k6/http/job_test.js) — this suite proves the seeded ids show up
// correctly rather than asserting an exact total record count. Per
// ADR-0024's "already known" short-circuit (#489), a fixture file already
// known from an earlier scan (this suite's own prior run, or
// test/k6/http/scan_test.js which runs first alphabetically) resolves
// matched_unmatched_file instead of queuing a new row — the id is read
// from whichever step actually carried it.
import http from 'k6/http';
import { check, sleep } from 'k6';
import { options } from '../lib/options.js';
export { options };

const BASE_URL = __ENV.PURSER_HTTP_URL || 'http://localhost:7474';
const FIXTURE_ROOT = __ENV.PURSER_SCAN_FIXTURE_ROOT || '/media/content/scan';
const FIXTURE_FILE_COUNT = parseInt(__ENV.PURSER_SCAN_FIXTURE_FILE_COUNT || '3', 10);
const SCAN_SERVICE = `${BASE_URL}/purser.pipeline.v1.ScanService`;
const JOB_SERVICE = `${BASE_URL}/purser.job.v1.JobService`;
const UNMATCHED_FILE_SERVICE = `${BASE_URL}/purser.pipeline.v1.UnmatchedFileService`;
const HEADERS = { headers: { 'Content-Type': 'application/json' } };

function invoke(url, body, headers) {
  const res = http.post(url, body, headers);
  console.log(JSON.stringify({ method: url, request: JSON.parse(body), response: res.json() }, null, 2));
  return res;
}

const terminal = ['JOB_STATUS_SUCCEEDED', 'JOB_STATUS_FAILED', 'JOB_STATUS_PARTIAL'];

function waitForTerminalJob(jobId) {
  let job = null;
  for (let i = 0; i < 100; i++) {
    const res = invoke(`${JOB_SERVICE}/GetJob`, JSON.stringify({ id: jobId }), HEADERS);
    check(res, { 'GetJob status is 200': (r) => r.status === 200 });
    job = res.json('job');
    if (terminal.indexOf(job.status) !== -1) {
      break;
    }
    sleep(0.1);
  }
  return job;
}

// unmatchedFileIdOf reads a task's unmatched_file.id regardless of which
// step actually carried it: a "new" outcome (#487) creates it in the
// "queue" step, while a "matched_unmatched_file" outcome (#489) reports
// the pre-existing id straight from "check_known" instead (no queue step
// runs at all). A "matched_media_file" outcome has no unmatched_file.id at
// all — that would mean this fixture file collided with some other
// suite's MediaFile, which none of these suites are supposed to leave
// behind (see test/k6/http/scan_test.js's cleanup).
function unmatchedFileIdOf(task) {
  const checkKnown = task.steps.find((s) => s.name === 'check_known');
  if (checkKnown.detail.outcome === 'matched_unmatched_file') {
    return checkKnown.detail['unmatched_file.id'];
  }
  const queue = task.steps.find((s) => s.name === 'queue');
  return queue && queue.detail['unmatched_file.id'];
}

function triggerScanAndCollectIds(root, wantCount) {
  const res = invoke(`${SCAN_SERVICE}/TriggerScan`, JSON.stringify({ root: root }), HEADERS);
  check(res, {
    'TriggerScan status is 200': (r) => r.status === 200,
    'TriggerScan returns a job id': (r) => !!r.json('jobId'),
  });
  const job = waitForTerminalJob(res.json('jobId'));
  check(job, {
    'scan job succeeded': (j) => j && j.status === 'JOB_STATUS_SUCCEEDED',
    [`scan job has ${wantCount} tasks`]: (j) => j && j.tasks && j.tasks.length === wantCount,
    'every task resolves to a real unmatched_file.id': (j) => j.tasks.every((t) => !!unmatchedFileIdOf(t)),
  });
  return job.tasks.map(unmatchedFileIdOf);
}

export default () => {
  const seededIds = triggerScanAndCollectIds(FIXTURE_ROOT, FIXTURE_FILE_COUNT);

  // GetUnmatchedFile on one of the seeded ids — the full hash set (osHash/
  // sha1 always populated; md5/sha512 depend on the server's hashing
  // toggles, so only presence of the always-on hashes is asserted here).
  let res = invoke(`${UNMATCHED_FILE_SERVICE}/GetUnmatchedFile`, JSON.stringify({ id: seededIds[0] }), HEADERS);
  check(res, {
    'GetUnmatchedFile status is 200': (r) => r.status === 200,
    'GetUnmatchedFile returns the requested id': (r) => r.json('unmatchedFile').id === seededIds[0],
    'GetUnmatchedFile returns osHash/sha1': (r) => !!r.json('unmatchedFile').osHash && !!r.json('unmatchedFile').sha1,
    'GetUnmatchedFile status is pending': (r) => r.json('unmatchedFile').status === 'UNMATCHED_FILE_STATUS_PENDING',
  });

  // ListUnmatchedFiles with no filter must include every seeded id, each
  // still pending.
  res = invoke(`${UNMATCHED_FILE_SERVICE}/ListUnmatchedFiles`, JSON.stringify({ pageSize: 50 }), HEADERS);
  check(res, {
    'ListUnmatchedFiles (no filter) status is 200': (r) => r.status === 200,
    'ListUnmatchedFiles (no filter) includes every seeded id': (r) => {
      const gotIds = (r.json('unmatchedFiles') || []).map((u) => u.id);
      return seededIds.every((id) => gotIds.indexOf(id) !== -1);
    },
    'ListUnmatchedFiles (no filter) seeded records are pending': (r) => {
      const byId = {};
      (r.json('unmatchedFiles') || []).forEach((u) => (byId[u.id] = u));
      return seededIds.every((id) => byId[id] && byId[id].status === 'UNMATCHED_FILE_STATUS_PENDING');
    },
  });

  // ListUnmatchedFiles filtered by status=pending must include every
  // seeded id, and every returned record must actually be pending.
  res = invoke(
    `${UNMATCHED_FILE_SERVICE}/ListUnmatchedFiles`,
    JSON.stringify({ pageSize: 50, status: 'UNMATCHED_FILE_STATUS_PENDING' }),
    HEADERS
  );
  check(res, {
    'ListUnmatchedFiles (status=pending) status is 200': (r) => r.status === 200,
    'ListUnmatchedFiles (status=pending) includes every seeded id': (r) => {
      const gotIds = (r.json('unmatchedFiles') || []).map((u) => u.id);
      return seededIds.every((id) => gotIds.indexOf(id) !== -1);
    },
    'ListUnmatchedFiles (status=pending) every result is pending': (r) =>
      (r.json('unmatchedFiles') || []).every((u) => u.status === 'UNMATCHED_FILE_STATUS_PENDING'),
  });

  // Pagination: page_size smaller than the total record count must page
  // through with no id repeated and no gap across the seeded ids. The
  // store is long-lived across k6 runs (like Job's), so this walks every
  // page rather than assuming the seeded records are the only ones.
  res = invoke(`${UNMATCHED_FILE_SERVICE}/ListUnmatchedFiles`, JSON.stringify({ pageSize: 2 }), HEADERS);
  check(res, {
    'ListUnmatchedFiles (page 1) status is 200': (r) => r.status === 200,
    'ListUnmatchedFiles (page 1) returns 2 records': (r) => (r.json('unmatchedFiles') || []).length === 2,
    'ListUnmatchedFiles (page 1) returns a next_page_token': (r) => !!r.json('nextPageToken'),
  });
  let allIds = (res.json('unmatchedFiles') || []).map((u) => u.id);
  let token = res.json('nextPageToken');
  while (token) {
    res = invoke(`${UNMATCHED_FILE_SERVICE}/ListUnmatchedFiles`, JSON.stringify({ pageSize: 2, pageToken: token }), HEADERS);
    check(res, { 'ListUnmatchedFiles (paging through) status is 200': (r) => r.status === 200 });
    allIds = allIds.concat((res.json('unmatchedFiles') || []).map((u) => u.id));
    token = res.json('nextPageToken');
  }
  check(
    { allIds: allIds },
    {
      'ListUnmatchedFiles paginated with no id repeated across pages': (v) => new Set(v.allIds).size === v.allIds.length,
      'ListUnmatchedFiles paginated through every seeded id, no gap': (v) => seededIds.every((id) => v.allIds.indexOf(id) !== -1),
    }
  );
};
