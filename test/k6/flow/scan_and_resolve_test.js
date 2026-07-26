// k6 flow: "discover and resolve a file" — the task a UI/admin tool runs
// when triaging the scan review queue. Scans a fixture directory the
// server can see, finds the file the scan just queued (or already had
// queued from an earlier run — see below), resolves it to a real Item, and
// proves the file is now a real MediaFile no longer sitting in the queue.
// This is the full chain from #487 through #491
// (docs/adr/0024-pipeline-core.md).
//
// PURSER_SCAN_FIXTURE_ROOT must point at a real directory the *server*
// process can see, containing at least one file >= 64 KiB (OSHash's
// minimum chunk size — see pkg/filehash). Default '/media/content/scan'
// matches ops/compose.yml's ../test-data bind mount; make k6-ci overrides
// it to the hermetic .cidata/scan fixture it creates itself. This is the
// same shared, long-lived fixture root test/k6/grpc/scan_test.js and
// test/k6/grpc/unmatched_file_test.js use — a first-ever TriggerScan here
// is not guaranteed to see "new" for every file, so this flow picks
// whichever discovered file is still awaiting a decision (outcome "new" or
// "matched_unmatched_file") rather than assuming a specific one is fresh.
// Teardown deletes the MediaFile/Item this flow creates, which returns
// that file's hash to "no known record" for the next run — the same
// cleanup contract those two suites already document.
import grpc from 'k6/net/grpc';
import { check, sleep, fail } from 'k6';
import { options } from '../lib/options.js';
export { options };

const ADDR = __ENV.PURSER_GRPC_ADDR || 'localhost:7474';
const FIXTURE_ROOT = __ENV.PURSER_SCAN_FIXTURE_ROOT || '/media/content/scan';

const client = new grpc.Client();
client.load(
  ['../../../proto'],
  'purser/pipeline/v1/scan.proto',
  'purser/pipeline/v1/unmatched_file.proto',
  'purser/domain/v1/item.proto',
  'purser/domain/v1/media_file.proto',
  'purser/job/v1/job.proto'
);

function invoke(method, request, label) {
  const res = client.invoke(method, request);
  const ok = check(res, { [`${label} status is OK`]: (r) => r && r.status === grpc.StatusOK });
  if (!ok) {
    fail(`${label} failed: ${res && res.status} ${res && res.error && res.error.message}`);
  }
  console.log(JSON.stringify({ method: method, request: request, response: res.message }, null, 2));
  return res.message;
}

const terminal = ['JOB_STATUS_SUCCEEDED', 'JOB_STATUS_FAILED', 'JOB_STATUS_PARTIAL'];

function waitForTerminalJob(jobId) {
  let job = null;
  for (let i = 0; i < 100; i++) {
    const message = invoke('purser.job.v1.JobService/GetJob', { id: jobId }, 'GetJob');
    job = message.job;
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

// unmatchedFileIdOf reads a task's unmatched_file.id regardless of which
// step carried it: a "new" outcome creates it in the "queue" step, while a
// "matched_unmatched_file" outcome reports the pre-existing id straight
// from "check_known" (no queue step runs). A "matched_media_file" outcome
// has no unmatched_file.id at all. Mirrors
// test/k6/grpc/unmatched_file_test.js's own helper.
function unmatchedFileIdOf(task) {
  const checkKnown = stepNamed(task, 'check_known');
  if (checkKnown.detail.outcome === 'matched_unmatched_file') {
    return checkKnown.detail['unmatched_file.id'];
  }
  const queue = stepNamed(task, 'queue');
  return queue && queue.detail['unmatched_file.id'];
}

export default () => {
  client.connect(ADDR, { plaintext: true });

  const scanRes = invoke('purser.pipeline.v1.ScanService/TriggerScan', { root: FIXTURE_ROOT }, 'TriggerScan');
  const job = waitForTerminalJob(scanRes.jobId);
  check(job, { 'scan job succeeded': (j) => j && j.status === 'JOB_STATUS_SUCCEEDED' });

  const queuedTask = (job.tasks || []).find((t) => !!unmatchedFileIdOf(t));
  if (!queuedTask) {
    fail('scan_and_resolve: no discovered file is awaiting resolution — every fixture file already resolved matched_media_file');
  }
  const unmatchedFileId = unmatchedFileIdOf(queuedTask);

  // The discovered file lands in the review queue.
  let res = invoke('purser.pipeline.v1.UnmatchedFileService/ListUnmatchedFiles', { pageSize: 50 }, 'ListUnmatchedFiles (before resolve)');
  check(res, {
    'discovered file appears in the pending queue': (r) => (r.unmatchedFiles || []).some((u) => u.id === unmatchedFileId),
  });

  // A real Item to resolve it to (ids are server-generated, per
  // docs/adr/0020-server-generated-kernel-entity-ids.md — read back from
  // CreateItem's response, never sent).
  const createdItem = invoke(
    'purser.domain.v1.ItemService/CreateItem',
    { item: { contentType: 'movie', libraryEntryId: 'k6-scan-resolve-entry', title: 'K6 Scan-and-Resolve Item', status: 'ITEM_STATUS_WANTED' } },
    'CreateItem'
  );
  const itemId = createdItem.item.id;

  const resolved = invoke(
    'purser.pipeline.v1.UnmatchedFileService/ResolveUnmatchedFile',
    { unmatchedFileId: unmatchedFileId, itemId: itemId },
    'ResolveUnmatchedFile (match)'
  );
  check(resolved, {
    'ResolveUnmatchedFile returns a MediaFile, not an UnmatchedFile': (r) => !!r.mediaFile && !r.unmatchedFile,
    'ResolveUnmatchedFile MediaFile is linked to the Item': (r) => r.mediaFile.itemId === itemId,
  });
  const mediaFileId = resolved.mediaFile.id;

  const gotMediaFile = invoke('purser.domain.v1.MediaFileService/GetMediaFile', { id: mediaFileId }, 'GetMediaFile');
  check(gotMediaFile, {
    'GetMediaFile returns the real record, linked to the Item': (r) => r.mediaFile.id === mediaFileId && r.mediaFile.itemId === itemId,
    "GetMediaFile carries the file's hashes": (r) => !!r.mediaFile.osHash && !!r.mediaFile.sha1,
  });

  res = invoke('purser.pipeline.v1.UnmatchedFileService/ListUnmatchedFiles', { pageSize: 50 }, 'ListUnmatchedFiles (after resolve)');
  check(res, {
    'resolved file no longer appears in the queue': (r) => !(r.unmatchedFiles || []).some((u) => u.id === unmatchedFileId),
  });

  // Teardown, reverse order.
  invoke('purser.domain.v1.MediaFileService/DeleteMediaFile', { id: mediaFileId }, 'DeleteMediaFile');
  invoke('purser.domain.v1.ItemService/DeleteItem', { id: itemId }, 'DeleteItem');

  client.close();
};
