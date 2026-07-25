// k6 gRPC suite for ScanService. See test/k6/grpc/job_test.js for the
// polling-until-terminal pattern this follows. This is the Common Scan
// Pipeline's (#486) walking skeleton (#487) plus the "already known"
// short-circuit (#489, docs/adr/0024-pipeline-core.md).
//
// PURSER_SCAN_FIXTURE_ROOT must point at a real directory the *server*
// process can see, containing at least one file >= 64 KiB (OSHash's
// minimum chunk size — see pkg/filehash). Default '/media/content/scan'
// matches ops/compose.yml's ../test-data bind mount; make k6-ci overrides
// it to the hermetic .cidata/scan fixture it creates itself.
//
// Neither k6 nor this fixture directory is under this suite's control, so
// a first-ever TriggerScan is not guaranteed to see "new" for every file:
// a prior k6 run against the same long-lived store (persistent Postgres),
// or another suite in this same run, may already have scanned these exact
// bytes. Every assertion below is written to hold either way — see
// docs/adr/0024-pipeline-core.md's "already known" short-circuit.
import grpc from 'k6/net/grpc';
import { check, sleep } from 'k6';
import { options } from '../lib/options.js';
export { options };

const ADDR = __ENV.PURSER_GRPC_ADDR || 'localhost:7474';
const FIXTURE_ROOT = __ENV.PURSER_SCAN_FIXTURE_ROOT || '/media/content/scan';
const FIXTURE_FILE_COUNT = parseInt(__ENV.PURSER_SCAN_FIXTURE_FILE_COUNT || '3', 10);

const client = new grpc.Client();
client.load(
  ['../../../proto'],
  'purser/pipeline/v1/scan.proto',
  'purser/pipeline/v1/unmatched_file.proto',
  'purser/domain/v1/item.proto',
  'purser/domain/v1/media_file.proto',
  'purser/job/v1/job.proto'
);

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

function triggerScan(root) {
  const res = invoke('purser.pipeline.v1.ScanService/TriggerScan', { root: root });
  check(res, {
    'TriggerScan status is OK': (r) => r && r.status === grpc.StatusOK,
    'TriggerScan returns a job id': (r) => r && r.message && !!r.message.jobId,
  });
  return waitForTerminalJob(res.message.jobId);
}

function stepNamed(task, name) {
  return (task.steps || []).find((s) => s.name === name);
}

export default () => {
  client.connect(ADDR, { plaintext: true });

  // First scan: proves the hash -> check_known -> (queue | short-circuit)
  // shape for every discovered file, tolerant of whichever outcome the
  // check_known step actually resolves to (see file header).
  const job1 = triggerScan(FIXTURE_ROOT);
  check(job1, {
    'scan job succeeded': (j) => j && j.status === 'JOB_STATUS_SUCCEEDED',
    [`scan job has ${FIXTURE_FILE_COUNT} tasks (one per fixture file)`]: (j) => j && j.tasks && j.tasks.length === FIXTURE_FILE_COUNT,
    'every hash step succeeded with oshash/sha1 detail': (j) =>
      j.tasks.every((t) => {
        const hash = stepNamed(t, 'hash');
        return hash && hash.status === 'JOB_STATUS_SUCCEEDED' && hash.detail && !!hash.detail.oshash && !!hash.detail.sha1;
      }),
    'every check_known step succeeded with a recognized outcome': (j) =>
      j.tasks.every((t) => {
        const check_known = stepNamed(t, 'check_known');
        return (
          check_known &&
          check_known.status === 'JOB_STATUS_SUCCEEDED' &&
          ['new', 'matched_media_file', 'matched_unmatched_file'].indexOf(check_known.detail.outcome) !== -1
        );
      }),
    'a queue step exists iff check_known resolved new, and never otherwise': (j) =>
      j.tasks.every((t) => {
        const outcome = stepNamed(t, 'check_known').detail.outcome;
        const queue = stepNamed(t, 'queue');
        return outcome === 'new' ? !!queue && queue.detail && !!queue.detail['unmatched_file.id'] : !queue;
      }),
  });

  // Capture each task's real sha1 (always computed) and whichever id
  // (media_file.id or unmatched_file.id) it currently resolves to, so the
  // second scan below can assert the record's Path actually moved.
  const byPath = {};
  job1.tasks.forEach((t) => {
    const outcome = stepNamed(t, 'check_known').detail.outcome;
    byPath[t.label] = { sha1: stepNamed(t, 'hash').detail.sha1, outcome: outcome };
  });

  // Simulate case 1 ("a file that matches an existing MediaFile by
  // hash — moved on disk"): create a real Item + MediaFile pointing at a
  // fabricated old path but the *real* sha1 of one already-discovered
  // fixture file. k6 cannot move files on disk itself (no fs-write
  // capability), so the "move" is modeled the way ADR-0024 actually
  // defines the short-circuit — purely by hash, not by path: the next
  // TriggerScan discovers that file again at its real (unchanged) path,
  // and the hash match alone must resolve matched_media_file and correct
  // the MediaFile's Path to where the file actually is.
  const targetPath = job1.tasks[0].label;
  const targetSHA1 = byPath[targetPath].sha1;

  let res = invoke('purser.domain.v1.ItemService/CreateItem', {
    item: { contentType: 'movie', libraryEntryId: 'k6-scan-entry', title: 'K6 Scan MediaFile Match', status: 'ITEM_STATUS_WANTED' },
  });
  check(res, { 'CreateItem status is OK': (r) => r && r.status === grpc.StatusOK });
  const itemId = res.message.item.id;

  res = invoke('purser.domain.v1.MediaFileService/CreateMediaFile', {
    mediaFile: { itemId: itemId, path: '/media/old-location/k6-scan-fixture.bin', sha1: targetSHA1 },
  });
  check(res, { 'CreateMediaFile status is OK': (r) => r && r.status === grpc.StatusOK });
  const mediaFileId = res.message.mediaFile.id;

  const job2 = triggerScan(FIXTURE_ROOT);
  check(job2, { 'second scan job succeeded': (j) => j && j.status === 'JOB_STATUS_SUCCEEDED' });

  const targetTask = job2.tasks.find((t) => t.label === targetPath);
  const targetCheck = stepNamed(targetTask, 'check_known');
  check(targetTask, {
    'MediaFile-match task: check_known resolves matched_media_file (MediaFile takes priority)': () =>
      targetCheck.detail.outcome === 'matched_media_file',
    'MediaFile-match task: check_known reports the created media_file.id': () => targetCheck.detail['media_file.id'] === mediaFileId,
    'MediaFile-match task: no queue step ran': () => !stepNamed(targetTask, 'queue'),
  });

  res = invoke('purser.domain.v1.MediaFileService/GetMediaFile', { id: mediaFileId });
  check(res, {
    'GetMediaFile status is OK': (r) => r && r.status === grpc.StatusOK,
    'GetMediaFile Path was updated to the real fixture path': (r) => r.message.mediaFile.path === targetPath,
  });

  // Every other fixture file must now resolve matched_unmatched_file —
  // case 2 ("a file that matches an existing UnmatchedFile by hash"),
  // since job1 either created or already found each of them queued.
  job2.tasks
    .filter((t) => t.label !== targetPath)
    .forEach((t) => {
      const c = stepNamed(t, 'check_known');
      check(t, {
        [`UnmatchedFile-match task ${t.label}: check_known resolves matched_unmatched_file`]: () =>
          c.detail.outcome === 'matched_unmatched_file',
        [`UnmatchedFile-match task ${t.label}: no queue step ran`]: () => !stepNamed(t, 'queue'),
      });
    });

  const otherTask = job2.tasks.find((t) => t.label !== targetPath);
  const otherUnmatchedFileId = stepNamed(otherTask, 'check_known').detail['unmatched_file.id'];
  res = invoke('purser.pipeline.v1.UnmatchedFileService/GetUnmatchedFile', { id: otherUnmatchedFileId });
  check(res, {
    'GetUnmatchedFile status is OK': (r) => r && r.status === grpc.StatusOK,
    'GetUnmatchedFile Path still matches the (unchanged) fixture path': (r) => r.message.unmatchedFile.path === otherTask.label,
  });

  // Clean up the synthetic MediaFile/Item so neither lingers to shadow
  // this fixture file for later suites/runs (test/k6/grpc/unmatched_file_test.js
  // reuses these same fixtures and expects every one of them collectible
  // as an UnmatchedFile id).
  res = invoke('purser.domain.v1.MediaFileService/DeleteMediaFile', { id: mediaFileId });
  check(res, { 'DeleteMediaFile (cleanup) status is OK': (r) => r && r.status === grpc.StatusOK });
  res = invoke('purser.domain.v1.ItemService/DeleteItem', { id: itemId });
  check(res, { 'DeleteItem (cleanup) status is OK': (r) => r && r.status === grpc.StatusOK });

  client.close();
};
