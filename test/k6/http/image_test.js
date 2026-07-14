// k6 HTTP/JSON suite for ImageService. See test/k6/http/person_test.js
// for the pattern this follows.
import http from 'k6/http';
import { check } from 'k6';

const BASE_URL = __ENV.PURSER_HTTP_URL || 'http://localhost:7474';
const SERVICE = `${BASE_URL}/purser.domain.v1.ImageService`;
const HEADERS = { headers: { 'Content-Type': 'application/json' } };

function invoke(url, body, headers) {
  const res = http.post(url, body, headers);
  console.log(JSON.stringify({ method: url, request: JSON.parse(body), response: res.json() }, null, 2));
  return res;
}

export default () => {
  // id is server-generated (docs/adr/0020-server-generated-kernel-entity-ids.md)
  // — never sent on Create, always read back from the response.
  const ownerId = 'k6-owner-1';

  let res = invoke(
    `${SERVICE}/CreateImage`,
    JSON.stringify({ image: { ownerType: 'person', ownerId: ownerId, imageType: 'poster', url: 'https://example.com/k6.jpg' } }),
    HEADERS
  );
  check(res, {
    'CreateImage status is 200': (r) => r.status === 200,
    'CreateImage returns an id': (r) => !!r.json('image.id'),
  });
  const id = res.json('image.id');

  res = invoke(`${SERVICE}/GetImage`, JSON.stringify({ id: id }), HEADERS);
  check(res, {
    'GetImage status is 200': (r) => r.status === 200,
    'GetImage returns the created url': (r) => r.json('image.url') === 'https://example.com/k6.jpg',
  });

  res = invoke(`${SERVICE}/UpdateImage`, JSON.stringify({ image: { id: id, url: 'https://example.com/k6-updated.jpg' }, updateMask: 'url' }), HEADERS);
  check(res, {
    'UpdateImage status is 200': (r) => r.status === 200,
    'UpdateImage applied the field-masked url': (r) => r.json('image.url') === 'https://example.com/k6-updated.jpg',
  });

  res = invoke(`${SERVICE}/ListImages`, JSON.stringify({ ownerType: 'person', ownerId: ownerId, pageSize: 10 }), HEADERS);
  check(res, {
    'ListImages status is 200': (r) => r.status === 200,
    'ListImages includes the created image': (r) => (r.json('images') || []).some((img) => img.id === id),
  });

  res = invoke(`${SERVICE}/DeleteImage`, JSON.stringify({ id: id }), HEADERS);
  check(res, { 'DeleteImage status is 200': (r) => r.status === 200 });

  res = invoke(`${SERVICE}/GetImage`, JSON.stringify({ id: id }), HEADERS);
  check(res, { 'GetImage after Delete is 404 (NotFound)': (r) => r.status === 404 });
};
