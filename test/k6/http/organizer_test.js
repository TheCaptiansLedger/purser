// k6 HTTP/JSON suite for OrganizerService (M11b) — see
// test/k6/grpc/organizer_test.js for the full rationale (organize.adult's
// template, why a real Studio LibraryEntry must be seeded for AfterDark's
// TemplateDataBuilder even though this template never renders any of its
// keys, the unique-per-run k6_marker, and why FIXTURE_ROOT is its own
// single-file .cidata/scan-organize-http root rather than
// PURSER_SCAN_FIXTURE_ROOT's shared .cidata/scan or the grpc variant's own
// isolated root). Reuses the exact TriggerScan-then-ResolveUnmatchedFile
// fixture pattern test/k6/http/unmatched_file_test.js already establishes.
import http from 'k6/http';
import { check, sleep } from 'k6';
import { options } from '../lib/options.js';
export { options };

const BASE_URL = __ENV.PURSER_HTTP_URL || 'http://localhost:7474';
const FIXTURE_ROOT = __ENV.PURSER_SCAN_ORGANIZE_HTTP_FIXTURE_ROOT || '/media/content/scan-organize-http';
const SCAN_SERVICE = `${BASE_URL}/purser.pipeline.v1.ScanService`;
const JOB_SERVICE = `${BASE_URL}/purser.job.v1.JobService`;
const UNMATCHED_FILE_SERVICE = `${BASE_URL}/purser.pipeline.v1.UnmatchedFileService`;
const ORGANIZER_SERVICE = `${BASE_URL}/purser.pipeline.v1.OrganizerService`;
const ITEM_SERVICE = `${BASE_URL}/purser.domain.v1.ItemService`;
const MEDIA_FILE_SERVICE = `${BASE_URL}/purser.domain.v1.MediaFileService`;
const LIBRARY_ENTRY_SERVICE = `${BASE_URL}/purser.domain.v1.LibraryEntryService`;
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

function unmatchedFileIdOf(task) {
  const checkKnown = task.steps.find((s) => s.name === 'check_known');
  if (checkKnown.detail.outcome === 'matched_unmatched_file') {
    return checkKnown.detail['unmatched_file.id'];
  }
  const queue = task.steps.find((s) => s.name === 'queue');
  return queue && queue.detail['unmatched_file.id'];
}

export default () => {
  let res = invoke(`${ORGANIZER_SERVICE}/Organize`, JSON.stringify({ mediaFileId: 'k6-http-no-such-media-file' }), HEADERS);
  check(res, { 'Organize with a nonexistent media_file_id is NotFound (404)': (r) => r.status === 404 });

  res = invoke(`${SCAN_SERVICE}/TriggerScan`, JSON.stringify({ root: FIXTURE_ROOT }), HEADERS);
  check(res, {
    'TriggerScan status is 200': (r) => r.status === 200,
    'TriggerScan returns a job id': (r) => !!r.json('jobId'),
  });
  const job = waitForTerminalJob(res.json('jobId'));
  check(job, { 'scan job succeeded': (j) => j && j.status === 'JOB_STATUS_SUCCEEDED' });
  const unmatchedFileId = unmatchedFileIdOf(job.tasks[0]);
  check({ unmatchedFileId: unmatchedFileId }, { 'scan produced a real unmatched_file.id': (v) => !!v.unmatchedFileId });

  // A real Studio LibraryEntry — item.LibraryEntryID must resolve to one,
  // per AfterDark's TemplateDataBuilder (see test/k6/grpc/organizer_test.js's
  // header comment).
  res = invoke(
    `${LIBRARY_ENTRY_SERVICE}/CreateLibraryEntry`,
    JSON.stringify({ libraryEntry: { contentType: 'adult', kind: 'studio', name: 'K6 Organize Studio', monitorMode: 'MONITOR_MODE_NONE' } }),
    HEADERS
  );
  check(res, { 'CreateLibraryEntry(studio) status is 200': (r) => r.status === 200 });
  const studioId = res.json('libraryEntry.id');

  const marker = `k6-organize-http-${__VU}-${__ITER}-${Date.now()}`;
  res = invoke(
    `${ITEM_SERVICE}/CreateItem`,
    JSON.stringify({
      item: {
        contentType: 'adult',
        libraryEntryId: studioId,
        title: 'K6 Organize Item',
        status: 'ITEM_STATUS_WANTED',
        metadata: { k6_marker: marker },
      },
    }),
    HEADERS
  );
  check(res, {
    'CreateItem status is 200': (r) => r.status === 200,
    'CreateItem returns an id': (r) => !!r.json('item.id'),
  });
  const itemId = res.json('item.id');

  res = invoke(`${UNMATCHED_FILE_SERVICE}/ResolveUnmatchedFile`, JSON.stringify({ unmatchedFileId: unmatchedFileId, itemId: itemId }), HEADERS);
  check(res, {
    'ResolveUnmatchedFile status is 200': (r) => r.status === 200,
    'ResolveUnmatchedFile returns a MediaFile': (r) => !!r.json('mediaFile'),
  });
  const mediaFileId = res.json('mediaFile.id');
  const originalPath = res.json('mediaFile.path');

  res = invoke(`${ORGANIZER_SERVICE}/Organize`, JSON.stringify({ mediaFileId: mediaFileId }), HEADERS);
  check(res, {
    'Organize status is 200': (r) => r.status === 200,
    'Organize returns the same media_file id': (r) => r.json('mediaFile.id') === mediaFileId,
    'Organize moved the file to a new path': (r) => r.json('mediaFile.path') !== originalPath,
    'Organize rendered the marker and extension into the destination path': (r) =>
      r.json('mediaFile.path').indexOf(marker) !== -1 && r.json('mediaFile.path').endsWith('.bin'),
  });
  const organizedPath = res.json('mediaFile.path');

  res = invoke(`${MEDIA_FILE_SERVICE}/GetMediaFile`, JSON.stringify({ id: mediaFileId }), HEADERS);
  check(res, {
    'GetMediaFile after Organize status is 200': (r) => r.status === 200,
    'GetMediaFile after Organize reflects the organized path': (r) => r.json('mediaFile.path') === organizedPath,
  });

  res = invoke(`${ORGANIZER_SERVICE}/Organize`, JSON.stringify({ mediaFileId: mediaFileId }), HEADERS);
  check(res, { 'Organize again onto its own already-organized destination is AlreadyExists (409)': (r) => r.status === 409 });

  res = invoke(`${MEDIA_FILE_SERVICE}/DeleteMediaFile`, JSON.stringify({ id: mediaFileId }), HEADERS);
  check(res, { 'DeleteMediaFile (cleanup) status is 200': (r) => r.status === 200 });
  res = invoke(`${ITEM_SERVICE}/DeleteItem`, JSON.stringify({ id: itemId }), HEADERS);
  check(res, { 'DeleteItem (cleanup) status is 200': (r) => r.status === 200 });
  res = invoke(`${LIBRARY_ENTRY_SERVICE}/DeleteLibraryEntry`, JSON.stringify({ id: studioId }), HEADERS);
  check(res, { 'DeleteLibraryEntry(studio) (cleanup) status is 200': (r) => r.status === 200 });
};
