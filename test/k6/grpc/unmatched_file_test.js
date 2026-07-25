// k6 gRPC suite for UnmatchedFileService. See test/k6/grpc/scan_test.js for
// the fixture/polling pattern this reuses to seed real UnmatchedFile rows —
// TriggerScan's check_known/queue step details (#487, #489) carry the ids
// this suite reads. UnmatchedFile is Datastore-backed (durable,
// docs/adr/0024-pipeline-core.md), so the server-side store accumulates
// across k6 runs the same way Job's in-memory store does (see
// test/k6/grpc/job_test.js) — this suite proves the seeded ids show up
// correctly rather than asserting an exact total record count. Per
// ADR-0024's "already known" short-circuit (#489), a fixture file already
// known from an earlier scan (this suite's own prior run, or
// test/k6/grpc/scan_test.js which runs first alphabetically) resolves
// matched_unmatched_file instead of queuing a new row — the id is read
// from whichever step actually carried it.
//
// This suite also exercises ResolveUnmatchedFile (#491) against two of
// the three seeded fixture files: one is matched (its MediaFile/Item are
// deleted again at the end, mirroring scan_test.js's own cleanup
// contract, so the fixture path is collectible as a fresh UnmatchedFile
// next run) and one is dismissed — dismiss is permanent by design, so a
// rerun of this suite sees that same file already dismissed; see
// statusOf/pendingSeededIds below for how the earlier list-completeness
// checks stay correct either way.
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
  'purser/job/v1/job.proto',
  'purser/domain/v1/item.proto',
  'purser/domain/v1/media_file.proto'
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

// unmatchedFileIdOf reads a task's unmatched_file.id regardless of which
// step actually carried it: a "new" outcome (#487) creates it in the
// "queue" step, while a "matched_unmatched_file" outcome (#489) reports
// the pre-existing id straight from "check_known" instead (no queue step
// runs at all). A "matched_media_file" outcome has no unmatched_file.id at
// all — that would mean this fixture file collided with some other
// suite's MediaFile, which none of these suites are supposed to leave
// behind (see test/k6/grpc/scan_test.js's cleanup).
function unmatchedFileIdOf(task) {
  const checkKnown = task.steps.find((s) => s.name === 'check_known');
  if (checkKnown.detail.outcome === 'matched_unmatched_file') {
    return checkKnown.detail['unmatched_file.id'];
  }
  const queue = task.steps.find((s) => s.name === 'queue');
  return queue && queue.detail['unmatched_file.id'];
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
    'every task resolves to a real unmatched_file.id': (j) => j.tasks.every((t) => !!unmatchedFileIdOf(t)),
  });
  return job.tasks.map(unmatchedFileIdOf);
}

export default () => {
  client.connect(ADDR, { plaintext: true });

  const seededIds = triggerScanAndCollectIds(FIXTURE_ROOT, FIXTURE_FILE_COUNT);

  // Below, this suite resolves seededIds[2] (dismiss — permanent, by
  // design) and seededIds[1] (match — cleaned up at the end of this run,
  // back to a fresh pending row next time). seededIds[0] is never touched.
  // A rerun of this suite against a long-lived store (the store
  // "accumulates across k6 runs", see file header) will therefore find
  // seededIds[2] already dismissed from a prior run — snapshotting each
  // seeded id's real current status here, rather than assuming every one
  // is freshly pending, keeps the list-completeness checks below correct
  // on both a first-ever run and a rerun.
  const statusOf = {};
  seededIds.forEach((id) => {
    const r = invoke('purser.pipeline.v1.UnmatchedFileService/GetUnmatchedFile', { id: id });
    check(r, { [`GetUnmatchedFile(${id}) status is OK`]: (rr) => rr && rr.status === grpc.StatusOK });
    statusOf[id] = r.message.unmatchedFile.status;
  });
  const pendingSeededIds = seededIds.filter((id) => statusOf[id] === 'UNMATCHED_FILE_STATUS_PENDING');

  // GetUnmatchedFile on seededIds[0] specifically — this suite never
  // resolves it, so it's always pending. The full hash set (OSHash/SHA1
  // always populated; MD5/SHA512 depend on the server's hashing toggles,
  // so only presence of the always-on hashes is asserted here).
  let res = invoke('purser.pipeline.v1.UnmatchedFileService/GetUnmatchedFile', { id: seededIds[0] });
  check(res, {
    'GetUnmatchedFile status is OK': (r) => r && r.status === grpc.StatusOK,
    'GetUnmatchedFile returns the requested id': (r) => r.message.unmatchedFile.id === seededIds[0],
    'GetUnmatchedFile returns osHash/sha1': (r) => !!r.message.unmatchedFile.osHash && !!r.message.unmatchedFile.sha1,
    'GetUnmatchedFile status is pending': (r) => r.message.unmatchedFile.status === 'UNMATCHED_FILE_STATUS_PENDING',
  });

  // ListUnmatchedFiles with no filter must include every seeded id
  // (dismissed rows are kept, not deleted — see docs/adr/0024-pipeline-core.md),
  // each with its real, previously-snapshotted status.
  res = invoke('purser.pipeline.v1.UnmatchedFileService/ListUnmatchedFiles', { pageSize: 50 });
  check(res, {
    'ListUnmatchedFiles (no filter) status is OK': (r) => r && r.status === grpc.StatusOK,
    'ListUnmatchedFiles (no filter) includes every seeded id': (r) => {
      const gotIds = (r.message.unmatchedFiles || []).map((u) => u.id);
      return seededIds.every((id) => gotIds.indexOf(id) !== -1);
    },
    'ListUnmatchedFiles (no filter) seeded records match their real status': (r) => {
      const byId = {};
      (r.message.unmatchedFiles || []).forEach((u) => (byId[u.id] = u));
      return seededIds.every((id) => byId[id] && byId[id].status === statusOf[id]);
    },
  });

  // ListUnmatchedFiles filtered by status=pending must include every
  // seeded id that's actually still pending, and every returned record
  // must actually be pending.
  res = invoke('purser.pipeline.v1.UnmatchedFileService/ListUnmatchedFiles', {
    pageSize: 50,
    status: 'UNMATCHED_FILE_STATUS_PENDING',
  });
  check(res, {
    'ListUnmatchedFiles (status=pending) status is OK': (r) => r && r.status === grpc.StatusOK,
    'ListUnmatchedFiles (status=pending) includes every still-pending seeded id': (r) => {
      const gotIds = (r.message.unmatchedFiles || []).map((u) => u.id);
      return pendingSeededIds.every((id) => gotIds.indexOf(id) !== -1);
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

  // ResolveUnmatchedFile: match. A real Item is created first (ids are
  // server-generated, per docs/adr/0020-server-generated-kernel-entity-ids.md
  // — read back from CreateItem's response, never sent).
  res = invoke('purser.domain.v1.ItemService/CreateItem', {
    item: { contentType: 'adult', libraryEntryId: 'k6-resolve-entry', title: 'K6 ResolveUnmatchedFile Item', status: 'ITEM_STATUS_WANTED' },
  });
  check(res, {
    'CreateItem status is OK': (r) => r && r.status === grpc.StatusOK,
    'CreateItem returns an id': (r) => r && r.message && r.message.item && !!r.message.item.id,
  });
  const itemId = res.message.item.id;
  const matchId = seededIds[1];

  res = invoke('purser.pipeline.v1.UnmatchedFileService/ResolveUnmatchedFile', { unmatchedFileId: matchId, itemId: itemId });
  check(res, {
    'ResolveUnmatchedFile (match) status is OK': (r) => r && r.status === grpc.StatusOK,
    'ResolveUnmatchedFile (match) returns a MediaFile, not an UnmatchedFile': (r) => r && r.message && !!r.message.mediaFile && !r.message.unmatchedFile,
    'ResolveUnmatchedFile (match) MediaFile is linked to the Item': (r) => r.message.mediaFile.itemId === itemId,
    'ResolveUnmatchedFile (match) MediaFile carries the file\'s hashes': (r) => !!r.message.mediaFile.osHash && !!r.message.mediaFile.sha1,
  });
  const mediaFileId = res.message.mediaFile.id;

  res = invoke('purser.domain.v1.MediaFileService/GetMediaFile', { id: mediaFileId });
  check(res, {
    'GetMediaFile after resolve (match) status is OK': (r) => r && r.status === grpc.StatusOK,
    'GetMediaFile after resolve (match) is linked to the Item': (r) => r.message.mediaFile.itemId === itemId,
    'GetMediaFile after resolve (match) carries the real path/hashes': (r) => !!r.message.mediaFile.path && !!r.message.mediaFile.osHash,
  });

  res = invoke('purser.pipeline.v1.UnmatchedFileService/ListUnmatchedFiles', { pageSize: 50 });
  check(res, {
    'ListUnmatchedFiles after resolve (match) no longer includes the resolved id': (r) =>
      !(r.message.unmatchedFiles || []).some((u) => u.id === matchId),
  });

  // ResolveUnmatchedFile: dismiss.
  const dismissId = seededIds[2];
  res = invoke('purser.pipeline.v1.UnmatchedFileService/ResolveUnmatchedFile', { unmatchedFileId: dismissId, dismiss: true });
  check(res, {
    'ResolveUnmatchedFile (dismiss) status is OK': (r) => r && r.status === grpc.StatusOK,
    'ResolveUnmatchedFile (dismiss) returns an UnmatchedFile, not a MediaFile': (r) => r && r.message && !!r.message.unmatchedFile && !r.message.mediaFile,
    'ResolveUnmatchedFile (dismiss) status is dismissed': (r) => r.message.unmatchedFile.status === 'UNMATCHED_FILE_STATUS_DISMISSED',
  });

  res = invoke('purser.pipeline.v1.UnmatchedFileService/GetUnmatchedFile', { id: dismissId });
  check(res, {
    'GetUnmatchedFile after resolve (dismiss) status is OK': (r) => r && r.status === grpc.StatusOK,
    'GetUnmatchedFile after resolve (dismiss) is dismissed': (r) => r.message.unmatchedFile.status === 'UNMATCHED_FILE_STATUS_DISMISSED',
  });

  // Rescan the same fixture root: the dismissed file (unchanged on disk)
  // must resolve via ADR-0024's "already known" short-circuit — matched by
  // hash against the same dismissed UnmatchedFile row, Path just
  // refreshed — rather than being silently re-queued as a new pending
  // entry. See #489's checkKnown, extended by this issue's acceptance
  // criteria to prove it's dismissed-aware.
  res = invoke('purser.pipeline.v1.ScanService/TriggerScan', { root: FIXTURE_ROOT });
  check(res, { 'rescan TriggerScan status is OK': (r) => r && r.status === grpc.StatusOK });
  const rescanJob = waitForTerminalJob(res.message.jobId);
  check(rescanJob, {
    'rescan job succeeded': (j) => j && j.status === 'JOB_STATUS_SUCCEEDED',
    'rescan resolves the dismissed file via the already-known short-circuit, same id': (j) =>
      j.tasks.some((t) => {
        const checkKnown = t.steps.find((s) => s.name === 'check_known');
        return checkKnown && checkKnown.detail.outcome === 'matched_unmatched_file' && checkKnown.detail['unmatched_file.id'] === dismissId;
      }),
  });

  res = invoke('purser.pipeline.v1.UnmatchedFileService/GetUnmatchedFile', { id: dismissId });
  check(res, {
    'GetUnmatchedFile after rescan is still dismissed (not silently re-queued)': (r) =>
      r && r.status === grpc.StatusOK && r.message.unmatchedFile.status === 'UNMATCHED_FILE_STATUS_DISMISSED',
  });
  res = invoke('purser.pipeline.v1.UnmatchedFileService/ListUnmatchedFiles', { pageSize: 50, status: 'UNMATCHED_FILE_STATUS_PENDING' });
  check(res, {
    'ListUnmatchedFiles (status=pending) after rescan does not include the dismissed id': (r) =>
      r && r.status === grpc.StatusOK && !(r.message.unmatchedFiles || []).some((u) => u.id === dismissId),
    'ListUnmatchedFiles (status=pending) after rescan still includes the untouched seeded id': (r) =>
      r && (r.message.unmatchedFiles || []).some((u) => u.id === seededIds[0]),
  });

  // ResolveUnmatchedFile with a nonexistent item_id — a real NotFound
  // error, not a silent no-op or a 500.
  res = invoke('purser.pipeline.v1.UnmatchedFileService/ResolveUnmatchedFile', {
    unmatchedFileId: seededIds[0],
    itemId: 'k6-grpc-no-such-item',
  });
  check(res, { 'ResolveUnmatchedFile with a nonexistent item_id is NotFound': (r) => r && r.status === grpc.StatusNotFound });

  // Clean up the match's MediaFile/Item — unlike dismiss (permanent, by
  // design), a match consumes one of the shared fixture files; deleting
  // both here reverts that path to "no known record" so the next run (or
  // test/k6/grpc/scan_test.js, which shares this same fixture root) sees
  // it as new/collectible again, the same contract scan_test.js's own
  // cleanup already documents.
  res = invoke('purser.domain.v1.MediaFileService/DeleteMediaFile', { id: mediaFileId });
  check(res, { 'DeleteMediaFile (cleanup) status is OK': (r) => r && r.status === grpc.StatusOK });
  res = invoke('purser.domain.v1.ItemService/DeleteItem', { id: itemId });
  check(res, { 'DeleteItem (cleanup) status is OK': (r) => r && r.status === grpc.StatusOK });

  client.close();
};
