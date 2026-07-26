// k6 flow (HTTP/JSON twin of scan_dedup_test.js): "the scanner never
// reimports what it already knows". See that script's header for the
// fixture-root, two-scenario, and cleanup-contract notes this flow shares.
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

// unmatchedFileIdOf mirrors scan_dedup_test.js's helper — see its comment
// for why a task's unmatched_file.id can live on either step.
function unmatchedFileIdOf(task) {
  const checkKnown = stepNamed(task, 'check_known');
  if (checkKnown.detail.outcome === 'matched_unmatched_file') {
    return checkKnown.detail['unmatched_file.id'];
  }
  const queue = stepNamed(task, 'queue');
  return queue && queue.detail['unmatched_file.id'];
}

function triggerScan(root) {
  const scanRes = invoke(`${SCAN_SERVICE}/TriggerScan`, { root: root }, 'TriggerScan');
  const job = waitForTerminalJob(scanRes.json('jobId'));
  check(job, { 'scan job succeeded': (j) => j && j.status === 'JOB_STATUS_SUCCEEDED' });
  return job;
}

export default () => {
  const job1 = triggerScan(FIXTURE_ROOT);

  // Two distinct fixture files still awaiting a decision — one plays the
  // "moved" scenario, the other the "dismissed" scenario. See file header
  // for why both must currently lack a MediaFile.
  const candidates = (job1.tasks || []).filter((t) => !!unmatchedFileIdOf(t));
  if (candidates.length < 2) {
    fail('scan_dedup: need at least two discovered files still awaiting a decision to run both scenarios');
  }
  const movedTask = candidates[0];
  const movedPath = movedTask.label;
  const movedSHA1 = stepNamed(movedTask, 'hash').detail.sha1;
  const dismissTask = candidates[1];
  const dismissId = unmatchedFileIdOf(dismissTask);

  // Scenario 1: moved file, simulated the way scan_test.js does — a real
  // Item + MediaFile at a fabricated old path but the real sha1 of an
  // already-discovered fixture file.
  const createdItem = invoke(
    `${ITEM_SERVICE}/CreateItem`,
    { item: { contentType: 'movie', libraryEntryId: 'k6-scan-dedup-entry', title: 'K6 Scan Dedup Item', status: 'ITEM_STATUS_WANTED' } },
    'CreateItem'
  );
  const itemId = createdItem.json('item.id');

  const createdMediaFile = invoke(
    `${MEDIA_FILE_SERVICE}/CreateMediaFile`,
    { mediaFile: { itemId: itemId, path: '/media/k6-dedup-old-location/moved.bin', sha1: movedSHA1 } },
    'CreateMediaFile'
  );
  const mediaFileId = createdMediaFile.json('mediaFile.id');

  // Scenario 2: dismiss a queued file.
  const dismissed = invoke(
    `${UNMATCHED_FILE_SERVICE}/ResolveUnmatchedFile`,
    { unmatchedFileId: dismissId, dismiss: true },
    'ResolveUnmatchedFile (dismiss)'
  );
  check(dismissed, {
    'ResolveUnmatchedFile (dismiss) status is dismissed': (r) => r.json('unmatchedFile') && r.json('unmatchedFile').status === 'UNMATCHED_FILE_STATUS_DISMISSED',
  });

  // Rescan, unchanged on disk: both files must resolve via the
  // hash-based "already known" short-circuit, not be re-queued.
  const job2 = triggerScan(FIXTURE_ROOT);

  const movedTask2 = job2.tasks.find((t) => t.label === movedPath);
  const movedCheck2 = stepNamed(movedTask2, 'check_known');
  check(movedTask2, {
    'moved file: check_known resolves matched_media_file on rescan': () => movedCheck2.detail.outcome === 'matched_media_file',
    'moved file: check_known reports the real media_file.id': () => movedCheck2.detail['media_file.id'] === mediaFileId,
    'moved file: no queue step ran — no new UnmatchedFile was created': () => !stepNamed(movedTask2, 'queue'),
  });

  const gotMediaFile = invoke(`${MEDIA_FILE_SERVICE}/GetMediaFile`, { id: mediaFileId }, 'GetMediaFile');
  check(gotMediaFile, {
    "moved file: MediaFile's Path was corrected to the real fixture path": (r) => r.json('mediaFile').path === movedPath,
  });

  const dismissTask2 = job2.tasks.find((t) => unmatchedFileIdOf(t) === dismissId);
  check(dismissTask2, {
    'dismissed file: rescan still resolves matched_unmatched_file to the same id': (t) =>
      !!t && stepNamed(t, 'check_known').detail.outcome === 'matched_unmatched_file',
  });

  const gotDismissed = invoke(`${UNMATCHED_FILE_SERVICE}/GetUnmatchedFile`, { id: dismissId }, 'GetUnmatchedFile');
  check(gotDismissed, {
    'dismissed file: still dismissed after rescan, not silently re-queued': (r) => r.json('unmatchedFile').status === 'UNMATCHED_FILE_STATUS_DISMISSED',
  });

  const pending = invoke(
    `${UNMATCHED_FILE_SERVICE}/ListUnmatchedFiles`,
    { pageSize: 50, status: 'UNMATCHED_FILE_STATUS_PENDING' },
    'ListUnmatchedFiles (status=pending)'
  );
  check(pending, {
    'dismissed file is still absent from the pending list after rescan': (r) => !(r.json('unmatchedFiles') || []).some((u) => u.id === dismissId),
  });

  // Teardown: the moved-file scenario's MediaFile/Item (returns that
  // fixture file to "no known record" for the next run). The dismissed
  // file stays dismissed, by design.
  invoke(`${MEDIA_FILE_SERVICE}/DeleteMediaFile`, { id: mediaFileId }, 'DeleteMediaFile');
  invoke(`${ITEM_SERVICE}/DeleteItem`, { id: itemId }, 'DeleteItem');
};
