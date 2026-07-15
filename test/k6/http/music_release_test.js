// k6 HTTP/JSON suite for MusicReleaseService — Connect's HTTP/JSON
// transport. See test/k6/http/group_test.js for the pattern this follows.
// Only Create/Get exist yet — this file is extended by every later
// sub-issue in the Music Release API epic, not replaced. See
// docs/adr/0021-music-domain-model.md.
import http from 'k6/http';
import { check } from 'k6';

const BASE_URL = __ENV.PURSER_HTTP_URL || 'http://localhost:7474';
const LIBRARY_ENTRY_SERVICE = `${BASE_URL}/purser.domain.v1.LibraryEntryService`;
const GROUP_SERVICE = `${BASE_URL}/purser.domain.v1.GroupService`;
const MUSIC_RELEASE_SERVICE = `${BASE_URL}/purser.music.v1.MusicReleaseService`;
const HEADERS = { headers: { 'Content-Type': 'application/json' } };

function invoke(url, body, headers) {
  const res = http.post(url, body, headers);
  console.log(JSON.stringify({ method: url, request: JSON.parse(body), response: res.json() }, null, 2));
  return res;
}

export default () => {
  // A MusicRelease requires a real LibraryEntry (artist) and Group (release
  // group) to attach to — created inline the same way library_entry_test.js
  // sets up a Group under a LibraryEntry.
  let res = invoke(
    `${LIBRARY_ENTRY_SERVICE}/CreateLibraryEntry`,
    JSON.stringify({ libraryEntry: { contentType: 'music', kind: 'artist', name: 'K6 HTTP Artist', monitorMode: 'MONITOR_MODE_NONE' } }),
    HEADERS
  );
  check(res, { 'CreateLibraryEntry status is 200': (r) => r.status === 200 });
  const libraryEntryId = res.json('libraryEntry.id');

  res = invoke(
    `${GROUP_SERVICE}/CreateGroup`,
    JSON.stringify({ group: { libraryEntryId: libraryEntryId, title: 'K6 HTTP Release Group', monitorMode: 'MONITOR_MODE_NONE' } }),
    HEADERS
  );
  check(res, { 'CreateGroup status is 200': (r) => r.status === 200 });
  const groupId = res.json('group.id');

  // ids are server-generated (docs/adr/0020-server-generated-kernel-entity-ids.md)
  // — never sent on Create, always read back from the response. A
  // caller-supplied id is explicitly sent here and must be discarded.
  res = invoke(
    `${MUSIC_RELEASE_SERVICE}/CreateMusicRelease`,
    JSON.stringify({
      musicRelease: {
        id: 'should-be-ignored',
        groupId: groupId,
        libraryEntryId: libraryEntryId,
        title: 'K6 HTTP Release',
        status: 'RELEASE_STATUS_STUB',
      },
    }),
    HEADERS
  );
  check(res, {
    'CreateMusicRelease status is 200': (r) => r.status === 200,
    'CreateMusicRelease returns an id': (r) => !!r.json('musicRelease.id'),
    'CreateMusicRelease discards the caller-supplied id': (r) => r.json('musicRelease.id') !== 'should-be-ignored',
  });
  const id = res.json('musicRelease.id');

  res = invoke(`${MUSIC_RELEASE_SERVICE}/GetMusicRelease`, JSON.stringify({ id: id }), HEADERS);
  check(res, {
    'GetMusicRelease status is 200': (r) => r.status === 200,
    'GetMusicRelease returns the created title': (r) => r.json('musicRelease.title') === 'K6 HTTP Release',
  });
};
