// k6 HTTP/JSON suite for MediaFileService. See test/k6/http/person_test.js
// for the pattern this follows.
import http from 'k6/http';
import { check } from 'k6';

const BASE_URL = __ENV.PURSER_HTTP_URL || 'http://localhost:7474';
const SERVICE = `${BASE_URL}/purser.domain.v1.MediaFileService`;
const HEADERS = { headers: { 'Content-Type': 'application/json' } };

function invoke(url, body, headers) {
  const res = http.post(url, body, headers);
  console.log(JSON.stringify({ method: url, request: JSON.parse(body), response: res.json() }, null, 2));
  return res;
}

export default () => {
  // id is server-generated (docs/adr/0020-server-generated-kernel-entity-ids.md)
  // — never sent on Create, always read back from the response.
  let res = invoke(`${SERVICE}/CreateMediaFile`, JSON.stringify({ mediaFile: { itemId: 'item1', path: '/media/k6.mkv' } }), HEADERS);
  check(res, {
    'CreateMediaFile status is 200': (r) => r.status === 200,
    'CreateMediaFile returns an id': (r) => !!r.json('mediaFile.id'),
  });
  const id = res.json('mediaFile.id');

  res = invoke(`${SERVICE}/GetMediaFile`, JSON.stringify({ id: id }), HEADERS);
  check(res, {
    'GetMediaFile status is 200': (r) => r.status === 200,
    'GetMediaFile returns the created path': (r) => r.json('mediaFile.path') === '/media/k6.mkv',
  });

  res = invoke(
    `${SERVICE}/UpdateMediaFile`,
    JSON.stringify({ mediaFile: { id: id, path: '/media/k6-updated.mkv' }, updateMask: 'path' }),
    HEADERS
  );
  check(res, {
    'UpdateMediaFile status is 200': (r) => r.status === 200,
    'UpdateMediaFile applied the field-masked path': (r) => r.json('mediaFile.path') === '/media/k6-updated.mkv',
  });

  res = invoke(`${SERVICE}/ListMediaFiles`, JSON.stringify({ pageSize: 10 }), HEADERS);
  check(res, {
    'ListMediaFiles status is 200': (r) => r.status === 200,
    'ListMediaFiles includes the created file': (r) => (r.json('mediaFiles') || []).some((m) => m.id === id),
  });

  res = invoke(`${SERVICE}/ListMediaFiles`, JSON.stringify({ itemId: 'item1', pageSize: 10 }), HEADERS);
  check(res, { 'ListMediaFiles filtered by itemId includes the created file': (r) => (r.json('mediaFiles') || []).some((m) => m.id === id) });

  res = invoke(`${SERVICE}/ListMediaFiles`, JSON.stringify({ itemId: 'no-such-item', pageSize: 10 }), HEADERS);
  check(res, {
    'ListMediaFiles filtered by a non-matching itemId excludes the created file': (r) => !(r.json('mediaFiles') || []).some((m) => m.id === id),
  });

  res = invoke(`${SERVICE}/DeleteMediaFile`, JSON.stringify({ id: id }), HEADERS);
  check(res, { 'DeleteMediaFile status is 200': (r) => r.status === 200 });

  res = invoke(`${SERVICE}/GetMediaFile`, JSON.stringify({ id: id }), HEADERS);
  check(res, { 'GetMediaFile after Delete is 404 (NotFound)': (r) => r.status === 404 });
};
