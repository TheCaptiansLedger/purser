// k6 HTTP/JSON suite for ItemService. See test/k6/http/person_test.js
// for the pattern this follows.
import http from 'k6/http';
import { check } from 'k6';

const BASE_URL = __ENV.PURSER_HTTP_URL || 'http://localhost:7474';
const SERVICE = `${BASE_URL}/purser.domain.v1.ItemService`;
const MEDIA_FILE = `${BASE_URL}/purser.domain.v1.MediaFileService`;
const HEADERS = { headers: { 'Content-Type': 'application/json' } };

function invoke(url, body, headers) {
  const res = http.post(url, body, headers);
  console.log(JSON.stringify({ method: url, request: JSON.parse(body), response: res.json() }, null, 2));
  return res;
}

export default () => {
  const id = `k6-http-${__VU}-${__ITER}-${Date.now()}`;
  const mediaFileId = `k6-http-item-media-${__VU}-${__ITER}-${Date.now()}`;

  let res = invoke(
    `${SERVICE}/CreateItem`,
    JSON.stringify({ item: { id: id, contentType: 'adult', libraryEntryId: 'entry1', title: 'K6 HTTP Item', status: 'ITEM_STATUS_WANTED' } }),
    HEADERS
  );
  check(res, {
    'CreateItem status is 200': (r) => r.status === 200,
    'CreateItem returns the id': (r) => r.json('item.id') === id,
  });

  res = invoke(`${SERVICE}/GetItem`, JSON.stringify({ id: id }), HEADERS);
  check(res, {
    'GetItem status is 200': (r) => r.status === 200,
    'GetItem returns the created title': (r) => r.json('item.title') === 'K6 HTTP Item',
  });

  res = invoke(`${SERVICE}/UpdateItem`, JSON.stringify({ item: { id: id, title: 'K6 HTTP Item Updated' }, updateMask: 'title' }), HEADERS);
  check(res, {
    'UpdateItem status is 200': (r) => r.status === 200,
    'UpdateItem applied the field-masked title': (r) => r.json('item.title') === 'K6 HTTP Item Updated',
  });

  res = invoke(`${SERVICE}/ListItems`, JSON.stringify({ pageSize: 10 }), HEADERS);
  check(res, {
    'ListItems status is 200': (r) => r.status === 200,
    'ListItems includes the created item': (r) => (r.json('items') || []).some((i) => i.id === id),
  });

  res = invoke(`${SERVICE}/ListItems`, JSON.stringify({ libraryEntryId: 'entry1', contentType: 'adult', pageSize: 10 }), HEADERS);
  check(res, {
    'ListItems filtered by libraryEntryId+contentType status is 200': (r) => r.status === 200,
    'ListItems filtered by libraryEntryId+contentType includes the created item': (r) => (r.json('items') || []).some((i) => i.id === id),
  });

  res = invoke(`${SERVICE}/ListItems`, JSON.stringify({ libraryEntryId: 'no-such-entry', pageSize: 10 }), HEADERS);
  check(res, {
    'ListItems filtered by a non-matching libraryEntryId excludes the created item': (r) => !(r.json('items') || []).some((i) => i.id === id),
  });

  res = invoke(`${MEDIA_FILE}/CreateMediaFile`, JSON.stringify({ mediaFile: { id: mediaFileId, itemId: id, path: '/media/k6-item-deletion.mkv' } }), HEADERS);
  check(res, { 'CreateMediaFile status is 200': (r) => r.status === 200 });

  res = invoke(`${SERVICE}/GetItemDeletionImpact`, JSON.stringify({ id: id }), HEADERS);
  check(res, {
    'GetItemDeletionImpact status is 200': (r) => r.status === 200,
    'GetItemDeletionImpact reports the media file': (r) => (r.json('impacts') || []).some((i) => i.kind === 'media_file' && i.count === 1),
  });

  res = invoke(`${SERVICE}/DeleteItem`, JSON.stringify({ id: id }), HEADERS);
  check(res, { 'DeleteItem status is 200': (r) => r.status === 200 });

  res = invoke(`${SERVICE}/GetItem`, JSON.stringify({ id: id }), HEADERS);
  check(res, { 'GetItem after Delete is 404 (NotFound)': (r) => r.status === 404 });

  res = invoke(`${MEDIA_FILE}/GetMediaFile`, JSON.stringify({ id: mediaFileId }), HEADERS);
  check(res, { 'GetMediaFile after Item Delete is 404 (unlinked)': (r) => r.status === 404 });

  // BulkDeleteItems: the "delete these 12 duplicate scenes" use case — one
  // of the two entities ADR 0016 names for a real bulk-delete endpoint.
  const bulkId1 = `k6-http-bulk-item-${__VU}-${__ITER}-${Date.now()}-1`;
  const bulkId2 = `k6-http-bulk-item-${__VU}-${__ITER}-${Date.now()}-2`;
  const bulkId3 = `k6-http-bulk-item-${__VU}-${__ITER}-${Date.now()}-3`;

  for (const bulkId of [bulkId1, bulkId2, bulkId3]) {
    res = invoke(
      `${SERVICE}/CreateItem`,
      JSON.stringify({ item: { id: bulkId, contentType: 'adult', libraryEntryId: 'entry1', title: 'K6 Bulk Item', status: 'ITEM_STATUS_WANTED' } }),
      HEADERS
    );
    check(res, { 'setup: CreateItem status is 200': (r) => r.status === 200 });
  }

  res = invoke(`${SERVICE}/BulkDeleteItems`, JSON.stringify({ ids: [bulkId1, bulkId2] }), HEADERS);
  check(res, { 'BulkDeleteItems status is 200': (r) => r.status === 200 });

  res = invoke(`${SERVICE}/GetItem`, JSON.stringify({ id: bulkId1 }), HEADERS);
  check(res, { 'GetItem for bulkId1 after BulkDeleteItems is 404': (r) => r.status === 404 });
  res = invoke(`${SERVICE}/GetItem`, JSON.stringify({ id: bulkId2 }), HEADERS);
  check(res, { 'GetItem for bulkId2 after BulkDeleteItems is 404': (r) => r.status === 404 });
  res = invoke(`${SERVICE}/GetItem`, JSON.stringify({ id: bulkId3 }), HEADERS);
  check(res, { 'GetItem for bulkId3 (not in the batch) still exists': (r) => r.status === 200 });

  // All-or-nothing: a batch with one missing id must fail entirely — the
  // still-existing bulkId3 must not be removed either.
  res = invoke(`${SERVICE}/BulkDeleteItems`, JSON.stringify({ ids: [bulkId3, 'k6-http-item-missing'] }), HEADERS);
  check(res, { 'BulkDeleteItems with a missing id is 404': (r) => r.status === 404 });

  res = invoke(`${SERVICE}/GetItem`, JSON.stringify({ id: bulkId3 }), HEADERS);
  check(res, { 'GetItem for bulkId3 after failed batch still exists (rolled back)': (r) => r.status === 200 });

  res = invoke(`${SERVICE}/DeleteItem`, JSON.stringify({ id: bulkId3 }), HEADERS);
  check(res, { 'cleanup: DeleteItem status is 200': (r) => r.status === 200 });
};
