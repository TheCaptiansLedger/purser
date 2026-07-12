// k6 HTTP/JSON suite for ImageService. See test/k6/http/person_test.js
// for the pattern this follows.
import http from 'k6/http';
import { check } from 'k6';

const BASE_URL = __ENV.PURSER_HTTP_URL || 'http://localhost:8080';
const SERVICE = `${BASE_URL}/purser.domain.v1.ImageService`;
const HEADERS = { headers: { 'Content-Type': 'application/json' } };

export default () => {
  const id = `k6-http-${__VU}-${__ITER}-${Date.now()}`;
  const ownerId = 'k6-owner-1';

  let res = http.post(
    `${SERVICE}/CreateImage`,
    JSON.stringify({ image: { id: id, ownerType: 'person', ownerId: ownerId, imageType: 'poster', url: 'https://example.com/k6.jpg' } }),
    HEADERS
  );
  check(res, {
    'CreateImage status is 200': (r) => r.status === 200,
    'CreateImage returns the id': (r) => r.json('image.id') === id,
  });

  res = http.post(`${SERVICE}/GetImage`, JSON.stringify({ id: id }), HEADERS);
  check(res, {
    'GetImage status is 200': (r) => r.status === 200,
    'GetImage returns the created url': (r) => r.json('image.url') === 'https://example.com/k6.jpg',
  });

  res = http.post(`${SERVICE}/UpdateImage`, JSON.stringify({ image: { id: id, url: 'https://example.com/k6-updated.jpg' }, updateMask: 'url' }), HEADERS);
  check(res, {
    'UpdateImage status is 200': (r) => r.status === 200,
    'UpdateImage applied the field-masked url': (r) => r.json('image.url') === 'https://example.com/k6-updated.jpg',
  });

  res = http.post(`${SERVICE}/ListImages`, JSON.stringify({ ownerType: 'person', ownerId: ownerId, pageSize: 10 }), HEADERS);
  check(res, {
    'ListImages status is 200': (r) => r.status === 200,
    'ListImages includes the created image': (r) => (r.json('images') || []).some((img) => img.id === id),
  });

  res = http.post(`${SERVICE}/DeleteImage`, JSON.stringify({ id: id }), HEADERS);
  check(res, { 'DeleteImage status is 200': (r) => r.status === 200 });

  res = http.post(`${SERVICE}/GetImage`, JSON.stringify({ id: id }), HEADERS);
  check(res, { 'GetImage after Delete is 404 (NotFound)': (r) => r.status === 404 });
};
