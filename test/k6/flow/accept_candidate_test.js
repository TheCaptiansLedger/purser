// k6 flow: the Music Persister's full cascade (#518,
// docs/technical/pipeline-music-persist.md) — both triggers, one
// Persister. purser serve must be started with musicbrainz.base_url
// pointed at internal/adapters/musicbrainz/fixtureserver (Makefile's
// _k6-app-start does this for `make k6-ci`) so this flow never touches
// the real MusicBrainz API — every provider adapter a k6 test exercises
// runs against fixture data, per docs/technical/pipeline-music-persist.md's
// own reasoning extended to the k6/CI harness.
//
// PURSER_SCAN_MUSIC_FIXTURE_ROOT must point at a directory the *server*
// process can see, containing test/k6/fixtures/musicbrainz-audio/'s three
// tagged FLAC fixtures (01/02 at the root, 03 under an ambiguous/
// subfolder — see that directory's generate.sh). Default
// '/media/content/music-scan' matches a would-be compose bind mount;
// make k6-ci overrides it to the hermetic .cidata/scan-music it creates.
//
// Two scenarios, deliberately not relying on TriggerScan's own timing:
// scan_roots being configured for this path already starts a live
// pkg/fswatch watcher (docs/adr/0024-pipeline-core.md) that auto-scans
// pre-existing files at server startup — this flow polls for each
// scenario's real end state instead of assuming which trigger (watcher or
// an explicit TriggerScan) got there first, since both are safe to race
// (identify/decide is idempotent, per docs/technical/pipeline-music-persist.md's
// "No true cross-repository transaction" section) and either is a
// realistic way a real deployment reaches the same state.
//
//   1. 01/02-*.flac carry MUSICBRAINZ_ALBUMID — M7's direct-ID
//      short-circuit resolves them with zero search calls, auto-importing
//      via the shared DecisionService with no human involved. This flow
//      polls GetMusicReleaseByMBID(ReleaseMBID) until it appears.
//   2. ambiguous/03-*.flac carries no MusicBrainz tags and won't fuzzy-
//      match anything (the fixture server's search routes always return
//      empty) — identify finds nothing, decide never persists, and the
//      file sits in the review queue until this flow calls
//      AcceptCandidate(groupKey, ReleaseMBID2) directly with a raw,
//      human-supplied MBID — the "already know the correct release, type
//      it in" path, exercised with no pre-existing MatchCandidate in the
//      group's Candidates at all.
import grpc from 'k6/net/grpc';
import { check, sleep, fail } from 'k6';
import { options } from '../lib/options.js';
export { options };

const ADDR = __ENV.PURSER_GRPC_ADDR || 'localhost:7474';
const FIXTURE_ROOT = __ENV.PURSER_SCAN_MUSIC_FIXTURE_ROOT || '/media/content/music-scan';

// Must match internal/adapters/musicbrainz/fixtureserver's MBID/title
// constants exactly — Go and JS can't share constants directly.
const RELEASE_MBID = '33333333-3333-3333-3333-333333333333';
const RELEASE_MBID_2 = '88888888-8888-8888-8888-888888888888';
const ALBUM_TITLE = 'K6 Fixture Album';
const ALBUM_TITLE_2 = 'K6 Ambiguous Album';
const AMBIGUOUS_FILENAME = '03-k6-ambiguous-track.flac';

const client = new grpc.Client();
client.load(
  ['../../../proto'],
  'purser/pipeline/v1/scan.proto',
  'purser/pipeline/v1/unmatched_file.proto',
  'purser/music/v1/release.proto',
  'purser/domain/v1/library_entry.proto',
  'purser/domain/v1/group.proto',
  'purser/domain/v1/item.proto',
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

// invokeAllowNotFound is invoke, but a NotFound response is a legitimate
// poll-and-retry outcome, not a hard failure — used while waiting for the
// automatic decide/persist pass or the watcher's own scan to catch up.
function invokeAllowNotFound(method, request) {
  const res = client.invoke(method, request);
  if (res && res.status === grpc.StatusNotFound) {
    return null;
  }
  check(res, { [`${method} status is OK`]: (r) => r && r.status === grpc.StatusOK });
  return res.message;
}

const terminal = ['JOB_STATUS_SUCCEEDED', 'JOB_STATUS_FAILED', 'JOB_STATUS_PARTIAL'];

// waitForTerminalJob polls GetJob until scanRes.jobId reaches a terminal
// status — informational only here (see the caller's comment on why this
// flow doesn't hard-fail on the job's own status).
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

// pollUntil retries fn (returning a truthy value on success, null/undefined
// while still waiting) every 0.5s for up to ~30s — generous enough for the
// watcher's own debounce window plus a full identify/score/decide pass.
function pollUntil(label, fn) {
  for (let i = 0; i < 60; i++) {
    const v = fn();
    if (v) {
      return v;
    }
    sleep(0.5);
  }
  fail(`${label}: condition never became true within 30s`);
  return undefined;
}

export default () => {
  client.connect(ADDR, { plaintext: true });

  // Explicit, self-contained trigger — safe to call whether or not the
  // scan_roots watcher already processed these files (check_known's
  // hash-based short-circuit makes a repeat scan of already-known files a
  // no-op; a fresh run, or a rerun after a prior run's teardown deleted
  // the resulting rows, re-queues and re-identifies them from scratch).
  // Not hard-failing on the job's own terminal status: two schedulers
  // (this explicit trigger and the watcher) racing to decide/persist the
  // exact same not-yet-imported group can leave the *losing* side's own
  // DeleteBatch call seeing already-deleted rows — a benign, job-status-only
  // artifact of get-or-create idempotency (docs/technical/pipeline-music-persist.md's
  // "No true cross-repository transaction" section), not a real failure;
  // the poll below is this flow's actual assertion of correctness.
  const scanRes = invoke('purser.pipeline.v1.ScanService/TriggerScan', { root: FIXTURE_ROOT }, 'TriggerScan');
  waitForTerminalJob(scanRes.jobId);

  // --- Scenario 1: automatic auto-import via direct-ID short-circuit ---
  const release = pollUntil('auto-import release appears', () =>
    invokeAllowNotFound('purser.music.v1.MusicReleaseService/GetMusicReleaseByMBID', { mbid: RELEASE_MBID })
  );
  check(release, {
    'auto-imported release has the fixture title': (m) => m.musicRelease.title === ALBUM_TITLE,
    'auto-imported release status is imported': (m) => m.musicRelease.status === 'RELEASE_STATUS_IMPORTED',
    'auto-imported release has both tracks': (m) => m.musicRelease.trackCount === 2 && m.musicRelease.mediumCount === 1,
  });
  const releaseId = release.musicRelease.id;
  const groupId = release.musicRelease.groupId;
  const libraryEntryId = release.musicRelease.libraryEntryId;

  const tracks = invoke(
    'purser.music.v1.MusicReleaseService/ListMusicReleaseTracks',
    { releaseId: releaseId, pageSize: 10 },
    'ListMusicReleaseTracks'
  );
  check(tracks, {
    'auto-imported release has exactly 2 tracks': (m) => m.tracks && m.tracks.length === 2,
    'every auto-imported track is imported and linked to the artist': (m) =>
      (m.tracks || []).every((t) => t.status === 'ITEM_STATUS_IMPORTED' && t.libraryEntryId === libraryEntryId && t.groupId === groupId),
  });

  // --- Scenario 2: manual AcceptCandidate with a raw, human-supplied MBID ---
  const ambiguous = pollUntil('ambiguous file appears in the review queue', () => {
    const m = invoke('purser.pipeline.v1.UnmatchedFileService/ListUnmatchedFiles', { pageSize: 50, status: 'UNMATCHED_FILE_STATUS_PENDING' }, 'ListUnmatchedFiles (poll ambiguous)');
    const uf = (m.unmatchedFiles || []).find((u) => u.path.indexOf(AMBIGUOUS_FILENAME) !== -1);
    return uf ? uf : null;
  });
  const groupKey = ambiguous.groupKey;

  check(ambiguous, { 'ambiguous file identify found no candidates': (u) => !u.candidates || u.candidates.length === 0 });

  invoke('purser.pipeline.v1.UnmatchedFileService/AcceptCandidate', { groupKey: groupKey, externalRef: RELEASE_MBID_2 }, 'AcceptCandidate (manual, raw MBID)');

  const release2 = pollUntil('manually-accepted release appears', () =>
    invokeAllowNotFound('purser.music.v1.MusicReleaseService/GetMusicReleaseByMBID', { mbid: RELEASE_MBID_2 })
  );
  check(release2, {
    'manually-accepted release has the fixture title': (m) => m.musicRelease.title === ALBUM_TITLE_2,
    'manually-accepted release status is imported': (m) => m.musicRelease.status === 'RELEASE_STATUS_IMPORTED',
  });
  const releaseId2 = release2.musicRelease.id;
  const groupId2 = release2.musicRelease.groupId;
  const libraryEntryId2 = release2.musicRelease.libraryEntryId;

  const afterAccept = invoke('purser.pipeline.v1.UnmatchedFileService/ListUnmatchedFiles', { pageSize: 50 }, 'ListUnmatchedFiles (after AcceptCandidate)');
  check(afterAccept, {
    'the accepted file left the review queue': (m) => !(m.unmatchedFiles || []).some((u) => u.path.indexOf(AMBIGUOUS_FILENAME) !== -1),
  });

  const tracks2 = invoke(
    'purser.music.v1.MusicReleaseService/ListMusicReleaseTracks',
    { releaseId: releaseId2, pageSize: 10 },
    'ListMusicReleaseTracks (manual)'
  );
  check(tracks2, {
    'manually-accepted release has exactly 1 track': (m) => m.tracks && m.tracks.length === 1,
    'the manually-accepted track is imported': (m) => m.tracks[0].status === 'ITEM_STATUS_IMPORTED',
  });

  // Teardown, reverse order, both scenarios — same convention
  // test/k6/flow/create_music_release_test.js already documents. .cidata/
  // is wiped by _k6-app-stop regardless, but this keeps the flow
  // self-contained against a real, long-lived server too.
  (tracks.tracks || []).forEach((t) => invoke('purser.domain.v1.ItemService/DeleteItem', { id: t.id }, 'DeleteItem (auto-import track)'));
  invoke('purser.music.v1.MusicReleaseService/DeleteMusicRelease', { id: releaseId }, 'DeleteMusicRelease (auto-import)');
  invoke('purser.domain.v1.GroupService/DeleteGroup', { id: groupId }, 'DeleteGroup (auto-import release group)');
  invoke('purser.domain.v1.LibraryEntryService/DeleteLibraryEntry', { id: libraryEntryId }, 'DeleteLibraryEntry (auto-import artist)');

  (tracks2.tracks || []).forEach((t) => invoke('purser.domain.v1.ItemService/DeleteItem', { id: t.id }, 'DeleteItem (manual track)'));
  invoke('purser.music.v1.MusicReleaseService/DeleteMusicRelease', { id: releaseId2 }, 'DeleteMusicRelease (manual)');
  invoke('purser.domain.v1.GroupService/DeleteGroup', { id: groupId2 }, 'DeleteGroup (manual release group)');
  invoke('purser.domain.v1.LibraryEntryService/DeleteLibraryEntry', { id: libraryEntryId2 }, 'DeleteLibraryEntry (manual artist)');

  client.close();
};
