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
//
// This suite also exercises ResolveUnmatchedFile (#491) against two of
// the three seeded fixture files: one is matched (its MediaFile/Item are
// deleted again at the end, mirroring scan_test.js's own cleanup
// contract, so the fixture path is collectible as a fresh UnmatchedFile
// next run) and one is dismissed, via DismissUnmatchedFileBatch (#508)
// rather than ResolveUnmatchedFile — dismiss is permanent by design, so a
// rerun of this suite sees that same file already dismissed; see
// statusOf/pendingSeededIds below for how the earlier list-completeness
// checks stay correct either way. ListGroupUnmatchedFiles (#508) is also
// exercised, against the untouched seeded file's own group-of-one (real
// multi-file grouping is M3, not built by the pipeline yet — see
// docs/technical/pipeline-unmatchedfile-grouping.md).
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
const ITEM_SERVICE = `${BASE_URL}/purser.domain.v1.ItemService`;
const MEDIA_FILE_SERVICE = `${BASE_URL}/purser.domain.v1.MediaFileService`;
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
    const r = invoke(`${UNMATCHED_FILE_SERVICE}/GetUnmatchedFile`, JSON.stringify({ id: id }), HEADERS);
    check(r, { [`GetUnmatchedFile(${id}) status is 200`]: (rr) => rr.status === 200 });
    statusOf[id] = r.json('unmatchedFile').status;
  });
  const pendingSeededIds = seededIds.filter((id) => statusOf[id] === 'UNMATCHED_FILE_STATUS_PENDING');

  // GetUnmatchedFile on seededIds[0] specifically — this suite never
  // resolves it, so it's always pending. The full hash set (osHash/sha1
  // always populated; md5/sha512 depend on the server's hashing toggles,
  // so only presence of the always-on hashes is asserted here).
  let res = invoke(`${UNMATCHED_FILE_SERVICE}/GetUnmatchedFile`, JSON.stringify({ id: seededIds[0] }), HEADERS);
  check(res, {
    'GetUnmatchedFile status is 200': (r) => r.status === 200,
    'GetUnmatchedFile returns the requested id': (r) => r.json('unmatchedFile').id === seededIds[0],
    'GetUnmatchedFile returns osHash/sha1': (r) => !!r.json('unmatchedFile').osHash && !!r.json('unmatchedFile').sha1,
    'GetUnmatchedFile status is pending': (r) => r.json('unmatchedFile').status === 'UNMATCHED_FILE_STATUS_PENDING',
  });
  const groupKey = res.json('unmatchedFile').groupKey;

  // ListGroupUnmatchedFiles (#508): grouping itself isn't computed by the
  // scan pipeline yet (M3), so GroupKey defaults to the file's own Path —
  // a group of one, by construction. Proving this still returns exactly
  // the one seeded row is the live-server-shaped coverage this RPC gets;
  // a real multi-row group is exercised at the Go datastore-contract
  // layer instead (internal/ports/unmatchedfiletest), since the pipeline
  // can't produce one yet.
  res = invoke(`${UNMATCHED_FILE_SERVICE}/ListGroupUnmatchedFiles`, JSON.stringify({ groupKey: groupKey }), HEADERS);
  check(res, {
    'ListGroupUnmatchedFiles status is 200': (r) => r.status === 200,
    'ListGroupUnmatchedFiles returns exactly the one file in this group': (r) =>
      (r.json('unmatchedFiles') || []).length === 1 && r.json('unmatchedFiles')[0].id === seededIds[0],
  });

  // ListUnmatchedFiles with no filter must include every seeded id
  // (dismissed rows are kept, not deleted — see docs/adr/0024-pipeline-core.md),
  // each with its real, previously-snapshotted status.
  res = invoke(`${UNMATCHED_FILE_SERVICE}/ListUnmatchedFiles`, JSON.stringify({ pageSize: 50 }), HEADERS);
  check(res, {
    'ListUnmatchedFiles (no filter) status is 200': (r) => r.status === 200,
    'ListUnmatchedFiles (no filter) includes every seeded id': (r) => {
      const gotIds = (r.json('unmatchedFiles') || []).map((u) => u.id);
      return seededIds.every((id) => gotIds.indexOf(id) !== -1);
    },
    'ListUnmatchedFiles (no filter) seeded records match their real status': (r) => {
      const byId = {};
      (r.json('unmatchedFiles') || []).forEach((u) => (byId[u.id] = u));
      return seededIds.every((id) => byId[id] && byId[id].status === statusOf[id]);
    },
  });

  // ListUnmatchedFiles filtered by status=pending must include every
  // seeded id that's actually still pending, and every returned record
  // must actually be pending.
  res = invoke(
    `${UNMATCHED_FILE_SERVICE}/ListUnmatchedFiles`,
    JSON.stringify({ pageSize: 50, status: 'UNMATCHED_FILE_STATUS_PENDING' }),
    HEADERS
  );
  check(res, {
    'ListUnmatchedFiles (status=pending) status is 200': (r) => r.status === 200,
    'ListUnmatchedFiles (status=pending) includes every still-pending seeded id': (r) => {
      const gotIds = (r.json('unmatchedFiles') || []).map((u) => u.id);
      return pendingSeededIds.every((id) => gotIds.indexOf(id) !== -1);
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

  // ResolveUnmatchedFile: match. A real Item is created first (ids are
  // server-generated, per docs/adr/0020-server-generated-kernel-entity-ids.md
  // — read back from CreateItem's response, never sent).
  res = invoke(
    `${ITEM_SERVICE}/CreateItem`,
    JSON.stringify({ item: { contentType: 'adult', libraryEntryId: 'k6-resolve-entry', title: 'K6 ResolveUnmatchedFile Item', status: 'ITEM_STATUS_WANTED' } }),
    HEADERS
  );
  check(res, {
    'CreateItem status is 200': (r) => r.status === 200,
    'CreateItem returns an id': (r) => !!r.json('item.id'),
  });
  const itemId = res.json('item.id');
  const matchId = seededIds[1];

  res = invoke(`${UNMATCHED_FILE_SERVICE}/ResolveUnmatchedFile`, JSON.stringify({ unmatchedFileId: matchId, itemId: itemId }), HEADERS);
  check(res, {
    'ResolveUnmatchedFile (match) status is 200': (r) => r.status === 200,
    'ResolveUnmatchedFile (match) returns a MediaFile, not an UnmatchedFile': (r) => !!r.json('mediaFile') && !r.json('unmatchedFile'),
    'ResolveUnmatchedFile (match) MediaFile is linked to the Item': (r) => r.json('mediaFile').itemId === itemId,
    "ResolveUnmatchedFile (match) MediaFile carries the file's hashes": (r) => !!r.json('mediaFile').osHash && !!r.json('mediaFile').sha1,
  });
  const mediaFileId = res.json('mediaFile').id;

  res = invoke(`${MEDIA_FILE_SERVICE}/GetMediaFile`, JSON.stringify({ id: mediaFileId }), HEADERS);
  check(res, {
    'GetMediaFile after resolve (match) status is 200': (r) => r.status === 200,
    'GetMediaFile after resolve (match) is linked to the Item': (r) => r.json('mediaFile').itemId === itemId,
    'GetMediaFile after resolve (match) carries the real path/hashes': (r) => !!r.json('mediaFile').path && !!r.json('mediaFile').osHash,
  });

  res = invoke(`${UNMATCHED_FILE_SERVICE}/ListUnmatchedFiles`, JSON.stringify({ pageSize: 50 }), HEADERS);
  check(res, {
    'ListUnmatchedFiles after resolve (match) no longer includes the resolved id': (r) =>
      !(r.json('unmatchedFiles') || []).some((u) => u.id === matchId),
  });

  // Dismiss, via DismissUnmatchedFileBatch (#508) rather than
  // ResolveUnmatchedFile(dismiss=true) — proves the new bulk RPC reaches
  // the same terminal state (permanent, kept-not-deleted) the rest of
  // this suite already relies on. A real multi-id batch is covered at the
  // Go datastore-contract layer; the live pipeline can't produce a
  // multi-file group yet (see the ListGroupUnmatchedFiles comment above).
  const dismissId = seededIds[2];
  res = invoke(`${UNMATCHED_FILE_SERVICE}/DismissUnmatchedFileBatch`, JSON.stringify({ unmatchedFileIds: [dismissId] }), HEADERS);
  check(res, {
    'DismissUnmatchedFileBatch status is 200': (r) => r.status === 200,
    'DismissUnmatchedFileBatch returns exactly the dismissed file': (r) =>
      (r.json('unmatchedFiles') || []).length === 1 && r.json('unmatchedFiles')[0].id === dismissId,
    'DismissUnmatchedFileBatch status is dismissed': (r) => r.json('unmatchedFiles')[0].status === 'UNMATCHED_FILE_STATUS_DISMISSED',
  });

  res = invoke(`${UNMATCHED_FILE_SERVICE}/GetUnmatchedFile`, JSON.stringify({ id: dismissId }), HEADERS);
  check(res, {
    'GetUnmatchedFile after resolve (dismiss) status is 200': (r) => r.status === 200,
    'GetUnmatchedFile after resolve (dismiss) is dismissed': (r) => r.json('unmatchedFile').status === 'UNMATCHED_FILE_STATUS_DISMISSED',
  });

  // Rescan the same fixture root: the dismissed file (unchanged on disk)
  // must resolve via ADR-0024's "already known" short-circuit — matched by
  // hash against the same dismissed UnmatchedFile row, Path just
  // refreshed — rather than being silently re-queued as a new pending
  // entry. See #489's checkKnown, extended by this issue's acceptance
  // criteria to prove it's dismissed-aware.
  res = invoke(`${SCAN_SERVICE}/TriggerScan`, JSON.stringify({ root: FIXTURE_ROOT }), HEADERS);
  check(res, { 'rescan TriggerScan status is 200': (r) => r.status === 200 });
  const rescanJob = waitForTerminalJob(res.json('jobId'));
  check(rescanJob, {
    'rescan job succeeded': (j) => j && j.status === 'JOB_STATUS_SUCCEEDED',
    'rescan resolves the dismissed file via the already-known short-circuit, same id': (j) =>
      j.tasks.some((t) => {
        const checkKnown = (t.steps || []).find((s) => s.name === 'check_known');
        return checkKnown && checkKnown.detail.outcome === 'matched_unmatched_file' && checkKnown.detail['unmatched_file.id'] === dismissId;
      }),
  });

  res = invoke(`${UNMATCHED_FILE_SERVICE}/GetUnmatchedFile`, JSON.stringify({ id: dismissId }), HEADERS);
  check(res, {
    'GetUnmatchedFile after rescan is still dismissed (not silently re-queued)': (r) =>
      r.status === 200 && r.json('unmatchedFile').status === 'UNMATCHED_FILE_STATUS_DISMISSED',
  });
  res = invoke(
    `${UNMATCHED_FILE_SERVICE}/ListUnmatchedFiles`,
    JSON.stringify({ pageSize: 50, status: 'UNMATCHED_FILE_STATUS_PENDING' }),
    HEADERS
  );
  check(res, {
    'ListUnmatchedFiles (status=pending) after rescan does not include the dismissed id': (r) =>
      r.status === 200 && !(r.json('unmatchedFiles') || []).some((u) => u.id === dismissId),
    'ListUnmatchedFiles (status=pending) after rescan still includes the untouched seeded id': (r) =>
      (r.json('unmatchedFiles') || []).some((u) => u.id === seededIds[0]),
  });

  // ResolveUnmatchedFile with a nonexistent item_id — a real NotFound
  // error, not a silent no-op or a 500.
  res = invoke(
    `${UNMATCHED_FILE_SERVICE}/ResolveUnmatchedFile`,
    JSON.stringify({ unmatchedFileId: seededIds[0], itemId: 'k6-http-no-such-item' }),
    HEADERS
  );
  check(res, { 'ResolveUnmatchedFile with a nonexistent item_id is NotFound (404)': (r) => r.status === 404 });

  // AcceptCandidate (#518) with an unknown group_key — a real NotFound
  // error, checked before this RPC ever dispatches to a content type's
  // Persister (which for a real group_key would mean a live MusicBrainz
  // lookup — deliberately not exercised here; the full auto-import/
  // manual-accept flow is covered by internal/adapters/pipeline/music's Go
  // fixture tests and this issue's own manual dev-instance verification
  // checklist, not live-network k6 CI).
  res = invoke(
    `${UNMATCHED_FILE_SERVICE}/AcceptCandidate`,
    JSON.stringify({ groupKey: 'k6-http-no-such-group', externalRef: 'k6-http-fake-mbid' }),
    HEADERS
  );
  check(res, { 'AcceptCandidate with an unknown group_key is NotFound (404)': (r) => r.status === 404 });

  // Clean up the match's MediaFile/Item — unlike dismiss (permanent, by
  // design), a match consumes one of the shared fixture files; deleting
  // both here reverts that path to "no known record" so the next run (or
  // test/k6/http/scan_test.js, which shares this same fixture root) sees
  // it as new/collectible again, the same contract scan_test.js's own
  // cleanup already documents.
  res = invoke(`${MEDIA_FILE_SERVICE}/DeleteMediaFile`, JSON.stringify({ id: mediaFileId }), HEADERS);
  check(res, { 'DeleteMediaFile (cleanup) status is 200': (r) => r.status === 200 });
  res = invoke(`${ITEM_SERVICE}/DeleteItem`, JSON.stringify({ id: itemId }), HEADERS);
  check(res, { 'DeleteItem (cleanup) status is 200': (r) => r.status === 200 });
};
