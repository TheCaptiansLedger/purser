// k6 gRPC suite for OrganizerService (M11b) — the manual-trigger surface
// for docs/adr/0024-pipeline-core.md's Organizer stage, always available
// regardless of config.Pipeline.AutoOrganize. See
// docs/technical/pipeline-music-organizer.md. Reuses the exact
// TriggerScan-then-ResolveUnmatchedFile fixture pattern
// test/k6/grpc/unmatched_file_test.js already establishes to get a real,
// already-imported MediaFile to organize — see that file for the
// scan/polling shape this borrows.
//
// The target Item is created with content_type "adult", which has no
// registered ports.TemplateDataBuilder — the rendered path therefore comes
// entirely from the generic Organizer's own Ext injection and raw
// Item/MediaFile metadata passthrough (docs/technical/pipeline-music-organizer.md's
// ".Metadata"/"default" additions), proving both work end-to-end against a
// live server independent of any content-type-specific builder. Make's
// _k6-app-start configures organize.adult.template as
// "{{.Metadata.k6_marker}}{{.Ext}}" specifically for this suite — a fresh,
// unique k6_marker each run avoids destination collisions on a rerun
// against a long-lived dev server (Make's own k6-ci wipes .cidata/ between
// runs, so this only matters outside CI).
import grpc from 'k6/net/grpc';
import { check, sleep } from 'k6';
import { options } from '../lib/options.js';
export { options };

const ADDR = __ENV.PURSER_GRPC_ADDR || 'localhost:7474';
const FIXTURE_ROOT = __ENV.PURSER_SCAN_FIXTURE_ROOT || '/media/content/scan';

const client = new grpc.Client();
client.load(
  ['../../../proto'],
  'purser/pipeline/v1/scan.proto',
  'purser/pipeline/v1/unmatched_file.proto',
  'purser/pipeline/v1/organizer.proto',
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

// unmatchedFileIdOf mirrors test/k6/grpc/unmatched_file_test.js's own
// helper — a "matched_unmatched_file" outcome reports the pre-existing id
// straight from "check_known", while a "new" outcome creates it in the
// "queue" step.
function unmatchedFileIdOf(task) {
  const checkKnown = task.steps.find((s) => s.name === 'check_known');
  if (checkKnown.detail.outcome === 'matched_unmatched_file') {
    return checkKnown.detail['unmatched_file.id'];
  }
  const queue = task.steps.find((s) => s.name === 'queue');
  return queue && queue.detail['unmatched_file.id'];
}

export default () => {
  client.connect(ADDR, { plaintext: true });

  // Organize against a nonexistent media_file_id — a real NotFound error,
  // proving the RPC's own plumbing/error-mapping without needing any
  // fixture data.
  let res = invoke('purser.pipeline.v1.OrganizerService/Organize', { mediaFileId: 'k6-grpc-no-such-media-file' });
  check(res, { 'Organize with a nonexistent media_file_id is NotFound': (r) => r && r.status === grpc.StatusNotFound });

  // Seed a real, already-imported MediaFile the same way
  // unmatched_file_test.js's own "match" flow does: scan a fixture root,
  // create an Item, resolve one pending UnmatchedFile against it.
  res = invoke('purser.pipeline.v1.ScanService/TriggerScan', { root: FIXTURE_ROOT });
  check(res, {
    'TriggerScan status is OK': (r) => r && r.status === grpc.StatusOK,
    'TriggerScan returns a job id': (r) => r && r.message && !!r.message.jobId,
  });
  const job = waitForTerminalJob(res.message.jobId);
  check(job, { 'scan job succeeded': (j) => j && j.status === 'JOB_STATUS_SUCCEEDED' });
  const unmatchedFileId = unmatchedFileIdOf(job.tasks[0]);
  check({ unmatchedFileId: unmatchedFileId }, { 'scan produced a real unmatched_file.id': (v) => !!v.unmatchedFileId });

  const marker = `k6-organize-grpc-${__VU}-${__ITER}-${Date.now()}`;
  res = invoke('purser.domain.v1.ItemService/CreateItem', {
    item: {
      contentType: 'adult',
      libraryEntryId: 'k6-organize-entry',
      title: 'K6 Organize Item',
      status: 'ITEM_STATUS_WANTED',
      metadata: { k6_marker: marker },
    },
  });
  check(res, {
    'CreateItem status is OK': (r) => r && r.status === grpc.StatusOK,
    'CreateItem returns an id': (r) => r && r.message && r.message.item && !!r.message.item.id,
  });
  const itemId = res.message.item.id;

  res = invoke('purser.pipeline.v1.UnmatchedFileService/ResolveUnmatchedFile', { unmatchedFileId: unmatchedFileId, itemId: itemId });
  check(res, {
    'ResolveUnmatchedFile status is OK': (r) => r && r.status === grpc.StatusOK,
    'ResolveUnmatchedFile returns a MediaFile': (r) => r && r.message && !!r.message.mediaFile,
  });
  const mediaFileId = res.message.mediaFile.id;
  const originalPath = res.message.mediaFile.path;

  // The real Organize call: renders organize.adult's configured template
  // ("{{.Metadata.k6_marker}}{{.Ext}}", see Makefile's _k6-app-start)
  // against this Item's own raw metadata, moves the file, and returns the
  // MediaFile with its updated Path.
  res = invoke('purser.pipeline.v1.OrganizerService/Organize', { mediaFileId: mediaFileId });
  check(res, {
    'Organize status is OK': (r) => r && r.status === grpc.StatusOK,
    'Organize returns the same media_file id': (r) => r.message.mediaFile.id === mediaFileId,
    'Organize moved the file to a new path': (r) => r.message.mediaFile.path !== originalPath,
    'Organize rendered the marker and extension into the destination path': (r) =>
      r.message.mediaFile.path.indexOf(marker) !== -1 && r.message.mediaFile.path.endsWith('.bin'),
  });
  const organizedPath = res.message.mediaFile.path;

  res = invoke('purser.domain.v1.MediaFileService/GetMediaFile', { id: mediaFileId });
  check(res, {
    'GetMediaFile after Organize status is OK': (r) => r && r.status === grpc.StatusOK,
    'GetMediaFile after Organize reflects the organized path': (r) => r.message.mediaFile.path === organizedPath,
  });

  // Calling Organize again on the same, already-organized file must refuse
  // rather than silently no-op or move it a second time — the destination
  // it would render to is identical (same marker), which the file is
  // already sitting at, so this hits the collision guard directly.
  res = invoke('purser.pipeline.v1.OrganizerService/Organize', { mediaFileId: mediaFileId });
  check(res, { 'Organize again onto its own already-organized destination is AlreadyExists': (r) => r && r.status === grpc.StatusAlreadyExists });

  // Cleanup — mirrors unmatched_file_test.js's own contract for a
  // consumed fixture file.
  res = invoke('purser.domain.v1.MediaFileService/DeleteMediaFile', { id: mediaFileId });
  check(res, { 'DeleteMediaFile (cleanup) status is OK': (r) => r && r.status === grpc.StatusOK });
  res = invoke('purser.domain.v1.ItemService/DeleteItem', { id: itemId });
  check(res, { 'DeleteItem (cleanup) status is OK': (r) => r && r.status === grpc.StatusOK });

  client.close();
};
