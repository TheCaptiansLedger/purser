// k6 flow: "the scanner never reimports what it already knows" — the
// dedup guarantees a UI/admin tool relies on so rescanning a library never
// creates duplicate work. Two scenarios against the same initial scan:
//
// 1. Moved file (#489): a file whose hash matches an existing MediaFile
//    (simulated — k6 has no fs-write capability, so this mirrors
//    test/k6/grpc/scan_test.js's approach: create a MediaFile at a
//    fabricated old path but the *real* sha1 of an already-discovered
//    fixture file) gets its Path corrected on rescan, with no new
//    UnmatchedFile created.
// 2. Dismissed file (#491 dismiss + #489's dismissed-aware short-circuit):
//    a queued file that's dismissed stays dismissed and absent from the
//    pending list across a rescan, rather than being silently re-queued.
//
// PURSER_SCAN_FIXTURE_ROOT must point at a real directory the *server*
// process can see, containing at least two files >= 64 KiB (OSHash's
// minimum chunk size — see pkg/filehash); this flow needs two distinct
// fixture files still awaiting a decision to run both scenarios
// independently. Default '/media/content/scan' matches
// ops/compose.yml's ../test-data bind mount; make k6-ci overrides it to
// the hermetic .cidata/scan fixture it creates itself — see
// test/k6/grpc/scan_test.js's header for the shared, long-lived-store
// reasoning this flow follows too. Teardown deletes the moved-file
// scenario's MediaFile/Item (returning that fixture file to "no known
// record" for the next run); the dismissed file stays dismissed by
// design, same contract test/k6/grpc/unmatched_file_test.js documents.
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

// unmatchedFileIdOf mirrors test/k6/grpc/unmatched_file_test.js's helper —
// see its comment for why a task's unmatched_file.id can live on either
// step.
function unmatchedFileIdOf(task) {
  const checkKnown = stepNamed(task, 'check_known');
  if (checkKnown.detail.outcome === 'matched_unmatched_file') {
    return checkKnown.detail['unmatched_file.id'];
  }
  const queue = stepNamed(task, 'queue');
  return queue && queue.detail['unmatched_file.id'];
}

function triggerScan(root) {
  const scanRes = invoke('purser.pipeline.v1.ScanService/TriggerScan', { root: root }, 'TriggerScan');
  const job = waitForTerminalJob(scanRes.jobId);
  check(job, { 'scan job succeeded': (j) => j && j.status === 'JOB_STATUS_SUCCEEDED' });
  return job;
}

export default () => {
  client.connect(ADDR, { plaintext: true });

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
    'purser.domain.v1.ItemService/CreateItem',
    { item: { contentType: 'movie', libraryEntryId: 'k6-scan-dedup-entry', title: 'K6 Scan Dedup Item', status: 'ITEM_STATUS_WANTED' } },
    'CreateItem'
  );
  const itemId = createdItem.item.id;

  const createdMediaFile = invoke(
    'purser.domain.v1.MediaFileService/CreateMediaFile',
    { mediaFile: { itemId: itemId, path: '/media/k6-dedup-old-location/moved.bin', sha1: movedSHA1 } },
    'CreateMediaFile'
  );
  const mediaFileId = createdMediaFile.mediaFile.id;

  // Scenario 2: dismiss a queued file.
  const dismissed = invoke(
    'purser.pipeline.v1.UnmatchedFileService/ResolveUnmatchedFile',
    { unmatchedFileId: dismissId, dismiss: true },
    'ResolveUnmatchedFile (dismiss)'
  );
  check(dismissed, {
    'ResolveUnmatchedFile (dismiss) status is dismissed': (r) => r.unmatchedFile && r.unmatchedFile.status === 'UNMATCHED_FILE_STATUS_DISMISSED',
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

  const gotMediaFile = invoke('purser.domain.v1.MediaFileService/GetMediaFile', { id: mediaFileId }, 'GetMediaFile');
  check(gotMediaFile, {
    "moved file: MediaFile's Path was corrected to the real fixture path": (r) => r.mediaFile.path === movedPath,
  });

  const dismissTask2 = job2.tasks.find((t) => unmatchedFileIdOf(t) === dismissId);
  check(dismissTask2, {
    'dismissed file: rescan still resolves matched_unmatched_file to the same id': (t) =>
      !!t && stepNamed(t, 'check_known').detail.outcome === 'matched_unmatched_file',
  });

  const gotDismissed = invoke('purser.pipeline.v1.UnmatchedFileService/GetUnmatchedFile', { id: dismissId }, 'GetUnmatchedFile');
  check(gotDismissed, {
    'dismissed file: still dismissed after rescan, not silently re-queued': (r) => r.unmatchedFile.status === 'UNMATCHED_FILE_STATUS_DISMISSED',
  });

  const pending = invoke(
    'purser.pipeline.v1.UnmatchedFileService/ListUnmatchedFiles',
    { pageSize: 50, status: 'UNMATCHED_FILE_STATUS_PENDING' },
    'ListUnmatchedFiles (status=pending)'
  );
  check(pending, {
    'dismissed file is still absent from the pending list after rescan': (r) => !(r.unmatchedFiles || []).some((u) => u.id === dismissId),
  });

  // Teardown: the moved-file scenario's MediaFile/Item (returns that
  // fixture file to "no known record" for the next run). The dismissed
  // file stays dismissed, by design.
  invoke('purser.domain.v1.MediaFileService/DeleteMediaFile', { id: mediaFileId }, 'DeleteMediaFile');
  invoke('purser.domain.v1.ItemService/DeleteItem', { id: itemId }, 'DeleteItem');

  client.close();
};
