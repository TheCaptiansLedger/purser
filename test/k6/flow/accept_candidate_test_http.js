// k6 flow (HTTP/JSON twin of accept_candidate_test.js): the Music
// Persister's full cascade (#518, docs/technical/pipeline-music-persist.md)
// — both triggers, one Persister. See that script's header for the fixture
// server / two-scenario design this mirrors exactly.
import http from 'k6/http';
import { check, sleep, fail } from 'k6';
import { options } from '../lib/options.js';
export { options };

const BASE_URL = __ENV.PURSER_HTTP_URL || 'http://localhost:7474';
const FIXTURE_ROOT = __ENV.PURSER_SCAN_MUSIC_FIXTURE_ROOT || '/media/content/music-scan';
const SCAN = `${BASE_URL}/purser.pipeline.v1.ScanService`;
const JOB = `${BASE_URL}/purser.job.v1.JobService`;
const UNMATCHED_FILE = `${BASE_URL}/purser.pipeline.v1.UnmatchedFileService`;
const MUSIC_RELEASE = `${BASE_URL}/purser.music.v1.MusicReleaseService`;
const LIBRARY_ENTRY = `${BASE_URL}/purser.domain.v1.LibraryEntryService`;
const GROUP = `${BASE_URL}/purser.domain.v1.GroupService`;
const ITEM = `${BASE_URL}/purser.domain.v1.ItemService`;
const HEADERS = { headers: { 'Content-Type': 'application/json' } };

// Must match internal/adapters/musicbrainz/fixtureserver's MBID/title
// constants exactly — Go and JS can't share constants directly.
const RELEASE_MBID = '33333333-3333-3333-3333-333333333333';
const RELEASE_MBID_2 = '88888888-8888-8888-8888-888888888888';
const ALBUM_TITLE = 'K6 Fixture Album';
const ALBUM_TITLE_2 = 'K6 Ambiguous Album';
const AMBIGUOUS_FILENAME = '03-k6-ambiguous-track.flac';

function invoke(url, body, label) {
  const res = http.post(url, JSON.stringify(body), HEADERS);
  const ok = check(res, { [`${label} status is 200`]: (r) => r.status === 200 });
  if (!ok) {
    fail(`${label} failed: ${res.status} ${res.body}`);
  }
  console.log(JSON.stringify({ method: url, request: body, response: res.json() }, null, 2));
  return res.json();
}

// invokeAllowNotFound is invoke, but a 404 response is a legitimate
// poll-and-retry outcome, not a hard failure.
function invokeAllowNotFound(url, body) {
  const res = http.post(url, JSON.stringify(body), HEADERS);
  if (res.status === 404) {
    return null;
  }
  check(res, { [`${url} status is 200`]: (r) => r.status === 200 });
  return res.json();
}

// pollUntil retries fn (returning a truthy value on success, null/undefined
// while still waiting) every 0.5s for up to ~30s.
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

const terminal = ['JOB_STATUS_SUCCEEDED', 'JOB_STATUS_FAILED', 'JOB_STATUS_PARTIAL'];

// waitForTerminalJob polls GetJob until jobId reaches a terminal status —
// informational only (see the caller's comment on why this flow doesn't
// hard-fail on the job's own status).
function waitForTerminalJob(jobId) {
  let job = null;
  for (let i = 0; i < 100; i++) {
    const m = invoke(`${JOB}/GetJob`, { id: jobId }, 'GetJob');
    job = m.job;
    if (terminal.indexOf(job.status) !== -1) {
      break;
    }
    sleep(0.1);
  }
  return job;
}

export default () => {
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
  const scanRes = invoke(`${SCAN}/TriggerScan`, { root: FIXTURE_ROOT }, 'TriggerScan');
  waitForTerminalJob(scanRes.jobId);

  // --- Scenario 1: automatic auto-import via direct-ID short-circuit ---
  const release = pollUntil('auto-import release appears', () => invokeAllowNotFound(`${MUSIC_RELEASE}/GetMusicReleaseByMBID`, { mbid: RELEASE_MBID }));
  check(release, {
    'auto-imported release has the fixture title': (m) => m.musicRelease.title === ALBUM_TITLE,
    'auto-imported release status is imported': (m) => m.musicRelease.status === 'RELEASE_STATUS_IMPORTED',
    'auto-imported release has both tracks': (m) => m.musicRelease.trackCount === 2 && m.musicRelease.mediumCount === 1,
  });
  const releaseId = release.musicRelease.id;
  const groupId = release.musicRelease.groupId;
  const libraryEntryId = release.musicRelease.libraryEntryId;

  const tracks = invoke(`${MUSIC_RELEASE}/ListMusicReleaseTracks`, { releaseId: releaseId, pageSize: 10 }, 'ListMusicReleaseTracks');
  check(tracks, {
    'auto-imported release has exactly 2 tracks': (m) => m.tracks && m.tracks.length === 2,
    'every auto-imported track is imported and linked to the artist': (m) =>
      (m.tracks || []).every((t) => t.status === 'ITEM_STATUS_IMPORTED' && t.libraryEntryId === libraryEntryId && t.groupId === groupId),
  });

  // --- Scenario 2: manual AcceptCandidate with a raw, human-supplied MBID ---
  const ambiguous = pollUntil('ambiguous file appears in the review queue', () => {
    const m = invoke(`${UNMATCHED_FILE}/ListUnmatchedFiles`, { pageSize: 50, status: 'UNMATCHED_FILE_STATUS_PENDING' }, 'ListUnmatchedFiles (poll ambiguous)');
    const uf = (m.unmatchedFiles || []).find((u) => u.path.indexOf(AMBIGUOUS_FILENAME) !== -1);
    return uf ? uf : null;
  });
  const groupKey = ambiguous.groupKey;

  check(ambiguous, { 'ambiguous file identify found no candidates': (u) => !u.candidates || u.candidates.length === 0 });

  invoke(`${UNMATCHED_FILE}/AcceptCandidate`, { groupKey: groupKey, externalRef: RELEASE_MBID_2 }, 'AcceptCandidate (manual, raw MBID)');

  const release2 = pollUntil('manually-accepted release appears', () => invokeAllowNotFound(`${MUSIC_RELEASE}/GetMusicReleaseByMBID`, { mbid: RELEASE_MBID_2 }));
  check(release2, {
    'manually-accepted release has the fixture title': (m) => m.musicRelease.title === ALBUM_TITLE_2,
    'manually-accepted release status is imported': (m) => m.musicRelease.status === 'RELEASE_STATUS_IMPORTED',
  });
  const releaseId2 = release2.musicRelease.id;
  const groupId2 = release2.musicRelease.groupId;
  const libraryEntryId2 = release2.musicRelease.libraryEntryId;

  const afterAccept = invoke(`${UNMATCHED_FILE}/ListUnmatchedFiles`, { pageSize: 50 }, 'ListUnmatchedFiles (after AcceptCandidate)');
  check(afterAccept, {
    'the accepted file left the review queue': (m) => !(m.unmatchedFiles || []).some((u) => u.path.indexOf(AMBIGUOUS_FILENAME) !== -1),
  });

  const tracks2 = invoke(`${MUSIC_RELEASE}/ListMusicReleaseTracks`, { releaseId: releaseId2, pageSize: 10 }, 'ListMusicReleaseTracks (manual)');
  check(tracks2, {
    'manually-accepted release has exactly 1 track': (m) => m.tracks && m.tracks.length === 1,
    'the manually-accepted track is imported': (m) => m.tracks[0].status === 'ITEM_STATUS_IMPORTED',
  });

  // Teardown, reverse order, both scenarios.
  (tracks.tracks || []).forEach((t) => invoke(`${ITEM}/DeleteItem`, { id: t.id }, 'DeleteItem (auto-import track)'));
  invoke(`${MUSIC_RELEASE}/DeleteMusicRelease`, { id: releaseId }, 'DeleteMusicRelease (auto-import)');
  invoke(`${GROUP}/DeleteGroup`, { id: groupId }, 'DeleteGroup (auto-import release group)');
  invoke(`${LIBRARY_ENTRY}/DeleteLibraryEntry`, { id: libraryEntryId }, 'DeleteLibraryEntry (auto-import artist)');

  (tracks2.tracks || []).forEach((t) => invoke(`${ITEM}/DeleteItem`, { id: t.id }, 'DeleteItem (manual track)'));
  invoke(`${MUSIC_RELEASE}/DeleteMusicRelease`, { id: releaseId2 }, 'DeleteMusicRelease (manual)');
  invoke(`${GROUP}/DeleteGroup`, { id: groupId2 }, 'DeleteGroup (manual release group)');
  invoke(`${LIBRARY_ENTRY}/DeleteLibraryEntry`, { id: libraryEntryId2 }, 'DeleteLibraryEntry (manual artist)');
};
