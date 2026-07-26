// k6 flow (HTTP/JSON twin of scan_and_resolve_test.js): "discover and
// resolve a file" — the task a UI/admin tool runs when triaging the scan
// review queue. See that script's header for the fixture-root and
// already-known-state notes this flow shares.
import http from 'k6/http';
import { check, sleep, fail } from 'k6';
import { options } from '../lib/options.js';
export { options };

const BASE_URL = __ENV.PURSER_HTTP_URL || 'http://localhost:7474';
const FIXTURE_ROOT = __ENV.PURSER_SCAN_FIXTURE_ROOT || '/media/content/scan';
const SCAN_SERVICE = `${BASE_URL}/purser.pipeline.v1.ScanService`;
const JOB_SERVICE = `${BASE_URL}/purser.job.v1.JobService`;
const ITEM_SERVICE = `${BASE_URL}/purser.domain.v1.ItemService`;
const MEDIA_FILE_SERVICE = `${BASE_URL}/purser.domain.v1.MediaFileService`;
const UNMATCHED_FILE_SERVICE = `${BASE_URL}/purser.pipeline.v1.UnmatchedFileService`;
const HEADERS = { headers: { 'Content-Type': 'application/json' } };

function invoke(url, body, label) {
  const res = http.post(url, JSON.stringify(body), HEADERS);
  const ok = check(res, { [`${label} status is 200`]: (r) => r.status === 200 });
  if (!ok) {
    fail(`${label} failed: ${res.status} ${res.body}`);
  }
  console.log(JSON.stringify({ method: url, request: body, response: res.json() }, null, 2));
  return res;
}

const terminal = ['JOB_STATUS_SUCCEEDED', 'JOB_STATUS_FAILED', 'JOB_STATUS_PARTIAL'];

function waitForTerminalJob(jobId) {
  let job = null;
  for (let i = 0; i < 100; i++) {
    const res = invoke(`${JOB_SERVICE}/GetJob`, { id: jobId }, 'GetJob');
    job = res.json('job');
    if (terminal.indexOf(job.status) !== -1) {
      break;
    }
    sleep(0.1);
  }
  return job;
}

function stepNamed(task, name) {
  return (task.steps || []).find((s) => s.name === name);
}

// unmatchedFileIdOf mirrors scan_and_resolve_test.js's helper — see its
// comment for why a task's unmatched_file.id can live on either step.
function unmatchedFileIdOf(task) {
  const checkKnown = stepNamed(task, 'check_known');
  if (checkKnown.detail.outcome === 'matched_unmatched_file') {
    return checkKnown.detail['unmatched_file.id'];
  }
  const queue = stepNamed(task, 'queue');
  return queue && queue.detail['unmatched_file.id'];
}

export default () => {
  const scanRes = invoke(`${SCAN_SERVICE}/TriggerScan`, { root: FIXTURE_ROOT }, 'TriggerScan');
  const job = waitForTerminalJob(scanRes.json('jobId'));
  check(job, { 'scan job succeeded': (j) => j && j.status === 'JOB_STATUS_SUCCEEDED' });

  const queuedTask = (job.tasks || []).find((t) => !!unmatchedFileIdOf(t));
  if (!queuedTask) {
    fail('scan_and_resolve: no discovered file is awaiting resolution — every fixture file already resolved matched_media_file');
  }
  const unmatchedFileId = unmatchedFileIdOf(queuedTask);

  // The discovered file lands in the review queue.
  let res = invoke(`${UNMATCHED_FILE_SERVICE}/ListUnmatchedFiles`, { pageSize: 50 }, 'ListUnmatchedFiles (before resolve)');
  check(res, {
    'discovered file appears in the pending queue': (r) => (r.json('unmatchedFiles') || []).some((u) => u.id === unmatchedFileId),
  });

  // A real Item to resolve it to (ids are server-generated, per
  // docs/adr/0020-server-generated-kernel-entity-ids.md — read back from
  // CreateItem's response, never sent).
  const createdItem = invoke(
    `${ITEM_SERVICE}/CreateItem`,
    { item: { contentType: 'movie', libraryEntryId: 'k6-scan-resolve-entry', title: 'K6 Scan-and-Resolve Item', status: 'ITEM_STATUS_WANTED' } },
    'CreateItem'
  );
  const itemId = createdItem.json('item.id');

  const resolved = invoke(
    `${UNMATCHED_FILE_SERVICE}/ResolveUnmatchedFile`,
    { unmatchedFileId: unmatchedFileId, itemId: itemId },
    'ResolveUnmatchedFile (match)'
  );
  check(resolved, {
    'ResolveUnmatchedFile returns a MediaFile, not an UnmatchedFile': (r) => !!r.json('mediaFile') && !r.json('unmatchedFile'),
    'ResolveUnmatchedFile MediaFile is linked to the Item': (r) => r.json('mediaFile').itemId === itemId,
  });
  const mediaFileId = resolved.json('mediaFile.id');

  const gotMediaFile = invoke(`${MEDIA_FILE_SERVICE}/GetMediaFile`, { id: mediaFileId }, 'GetMediaFile');
  check(gotMediaFile, {
    'GetMediaFile returns the real record, linked to the Item': (r) => r.json('mediaFile').id === mediaFileId && r.json('mediaFile').itemId === itemId,
    "GetMediaFile carries the file's hashes": (r) => !!r.json('mediaFile').osHash && !!r.json('mediaFile').sha1,
  });

  res = invoke(`${UNMATCHED_FILE_SERVICE}/ListUnmatchedFiles`, { pageSize: 50 }, 'ListUnmatchedFiles (after resolve)');
  check(res, {
    'resolved file no longer appears in the queue': (r) => !(r.json('unmatchedFiles') || []).some((u) => u.id === unmatchedFileId),
  });

  // Teardown, reverse order.
  invoke(`${MEDIA_FILE_SERVICE}/DeleteMediaFile`, { id: mediaFileId }, 'DeleteMediaFile');
  invoke(`${ITEM_SERVICE}/DeleteItem`, { id: itemId }, 'DeleteItem');
};
