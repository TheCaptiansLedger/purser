// k6 flow: proves Music's real TemplateDataBuilder renders every field
// (ArtistName/AlbumTitle/Year/DiscCount/TrackTitle/Ext) end-to-end through
// a live server — nothing else in this suite does.
// test/k6/grpc/organizer_test.js (M11b) deliberately uses content_type
// "adult" (no registered TemplateDataBuilder) to test the generic
// Organizer mechanics in isolation; test/k6/flow/accept_candidate_test.js
// covers the Persister cascade (#518) but never calls Organize at all.
//
// Scans its own isolated .cidata/scan-music-organize root — a copy of
// test/k6/fixtures/musicbrainz-audio/organize/'s two fixtures, which carry
// the exact same MUSICBRAINZ_ALBUMID/tags as accept_candidate_test.js's
// own 01/02 fixtures (so they identify and auto-import the same way) but
// genuinely distinct audio bytes, so their hash never collides with
// scan-music's own files. This has to be a separate root, not a reuse of
// scan-music directly: Organize physically moves the file, and
// accept_candidate_test.js/accept_candidate_test_http.js both scan
// scan-music later in the same k6-ci invocation — organizing a file out
// from under it would silently break whichever of those runs second. See
// Make's _k6-app-start comment for the full reasoning, including why
// config.Pipeline.AutoOrganize itself stays off (a single global toggle —
// Organize is called explicitly here instead).
import grpc from 'k6/net/grpc';
import { check, sleep, fail } from 'k6';
import { options } from '../lib/options.js';
export { options };

const ADDR = __ENV.PURSER_GRPC_ADDR || 'localhost:7474';
const FIXTURE_ROOT = __ENV.PURSER_SCAN_MUSIC_ORGANIZE_FIXTURE_ROOT || '/media/content/music-scan-organize';

// Must match internal/adapters/musicbrainz/fixtureserver's MBID/title
// constants exactly — Go and JS can't share constants directly.
const RELEASE_MBID = '33333333-3333-3333-3333-333333333333';
const ALBUM_TITLE = 'K6 Fixture Album';
const TRACK1_TITLE = 'K6 Fixture Track One';
// Must match Make's _k6-app-start-generated organize.music.template
// exactly (ArtistName/AlbumTitle (Year)/TrackNumber - TrackTitle.Ext, no
// zero-padding — see that target's own comment on why).
const EXPECTED_ORGANIZED_SUFFIX = 'K6 Fixture Artist/K6 Fixture Album (2024)/1 - K6 Fixture Track One.flac';

const client = new grpc.Client();
client.load(
  ['../../../proto'],
  'purser/pipeline/v1/scan.proto',
  'purser/pipeline/v1/organizer.proto',
  'purser/music/v1/release.proto',
  'purser/domain/v1/library_entry.proto',
  'purser/domain/v1/group.proto',
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

// invokeAllowNotFound is invoke, but a NotFound response is a legitimate
// poll-and-retry outcome, not a hard failure — see
// accept_candidate_test.js's own copy of this helper for why.
function invokeAllowNotFound(method, request) {
  const res = client.invoke(method, request);
  if (res && res.status === grpc.StatusNotFound) {
    return null;
  }
  check(res, { [`${method} status is OK`]: (r) => r && r.status === grpc.StatusOK });
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

  // Explicit, self-contained trigger — see accept_candidate_test.js's own
  // comment on why this doesn't hard-fail on the job's own terminal
  // status (the watcher racing this same trigger is a benign,
  // get-or-create-idempotent outcome, not a real failure).
  const scanRes = invoke('purser.pipeline.v1.ScanService/TriggerScan', { root: FIXTURE_ROOT }, 'TriggerScan');
  waitForTerminalJob(scanRes.jobId);

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
  const track1 = (tracks.tracks || []).find((t) => t.title === TRACK1_TITLE);
  check(track1, { 'track 1 (by title) found in ListMusicReleaseTracks': (t) => !!t });

  const track1Media = invoke(
    'purser.domain.v1.MediaFileService/ListMediaFiles',
    { itemId: track1.id, pageSize: 1 },
    'ListMediaFiles (track 1)'
  );
  check(track1Media, { 'track 1 has exactly one MediaFile': (m) => m.mediaFiles && m.mediaFiles.length === 1 });

  // The actual point of this flow: Organize renders Music's real
  // TemplateDataBuilder data (ArtistName/AlbumTitle/Year/DiscCount/
  // TrackTitle/Ext, all pulled from the real persisted Item/Group/
  // MusicRelease/LibraryEntry chain) against a live server, not a unit
  // test double.
  const organized = invoke(
    'purser.pipeline.v1.OrganizerService/Organize',
    { mediaFileId: track1Media.mediaFiles[0].id },
    'Organize (track 1)'
  );
  check(organized, {
    'organized path ends with the expected Music-rendered destination': (m) =>
      m.mediaFile && m.mediaFile.path && m.mediaFile.path.endsWith(EXPECTED_ORGANIZED_SUFFIX),
  });

  // Teardown — same convention every other flow file here follows.
  (tracks.tracks || []).forEach((t) => invoke('purser.domain.v1.ItemService/DeleteItem', { id: t.id }, 'DeleteItem (track)'));
  invoke('purser.music.v1.MusicReleaseService/DeleteMusicRelease', { id: releaseId }, 'DeleteMusicRelease');
  invoke('purser.domain.v1.GroupService/DeleteGroup', { id: groupId }, 'DeleteGroup (release group)');
  invoke('purser.domain.v1.LibraryEntryService/DeleteLibraryEntry', { id: libraryEntryId }, 'DeleteLibraryEntry (artist)');

  client.close();
};
