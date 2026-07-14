// k6 HTTP/JSON suite for TagService. See test/k6/http/person_test.js for
// the pattern this follows.
import http from 'k6/http';
import { check } from 'k6';

const BASE_URL = __ENV.PURSER_HTTP_URL || 'http://localhost:7474';
const SERVICE = `${BASE_URL}/purser.domain.v1.TagService`;
const TAG_ASSIGNMENT = `${BASE_URL}/purser.domain.v1.TagAssignmentService`;
const HEADERS = { headers: { 'Content-Type': 'application/json' } };

function invoke(url, body, headers) {
  const res = http.post(url, body, headers);
  console.log(JSON.stringify({ method: url, request: JSON.parse(body), response: res.json() }, null, 2));
  return res;
}

export default () => {
  const id = `k6-http-${__VU}-${__ITER}-${Date.now()}`;
  const entityId = 'k6-tag-deletion-entity-1';

  let res = invoke(`${SERVICE}/CreateTag`, JSON.stringify({ tag: { id: id, key: 'genre', value: 'gonzo', scope: 'TAG_SCOPE_METADATA' } }), HEADERS);
  check(res, {
    'CreateTag status is 200': (r) => r.status === 200,
    'CreateTag returns the id': (r) => r.json('tag.id') === id,
  });

  res = invoke(`${SERVICE}/GetTag`, JSON.stringify({ id: id }), HEADERS);
  check(res, {
    'GetTag status is 200': (r) => r.status === 200,
    'GetTag returns the created value': (r) => r.json('tag.value') === 'gonzo',
  });

  res = invoke(`${SERVICE}/UpdateTag`, JSON.stringify({ tag: { id: id, value: 'action' }, updateMask: 'value' }), HEADERS);
  check(res, {
    'UpdateTag status is 200': (r) => r.status === 200,
    'UpdateTag applied the field-masked value': (r) => r.json('tag.value') === 'action',
  });

  res = invoke(`${SERVICE}/ListTags`, JSON.stringify({ pageSize: 10 }), HEADERS);
  check(res, {
    'ListTags status is 200': (r) => r.status === 200,
    'ListTags includes the created tag': (r) => (r.json('tags') || []).some((t) => t.id === id),
  });

  res = invoke(`${TAG_ASSIGNMENT}/CreateTagAssignment`, JSON.stringify({ tagAssignment: { tagId: id, entityType: 'ENTITY_TYPE_PERSON', entityId: entityId } }), HEADERS);
  check(res, { 'CreateTagAssignment status is 200': (r) => r.status === 200 });

  res = invoke(`${SERVICE}/GetTagDeletionImpact`, JSON.stringify({ id: id }), HEADERS);
  check(res, {
    'GetTagDeletionImpact status is 200': (r) => r.status === 200,
    'GetTagDeletionImpact reports the tag assignment': (r) =>
      (r.json('impacts') || []).some((i) => i.kind === 'tag_assignment' && i.count === 1),
  });

  res = invoke(`${SERVICE}/DeleteTag`, JSON.stringify({ id: id }), HEADERS);
  check(res, { 'DeleteTag status is 200': (r) => r.status === 200 });

  res = invoke(`${SERVICE}/GetTag`, JSON.stringify({ id: id }), HEADERS);
  check(res, { 'GetTag after Delete is 404 (NotFound)': (r) => r.status === 404 });

  res = invoke(`${TAG_ASSIGNMENT}/GetTagAssignment`, JSON.stringify({ tagId: id, entityType: 'ENTITY_TYPE_PERSON', entityId: entityId }), HEADERS);
  check(res, { 'GetTagAssignment after Tag Delete is 404 (unlinked)': (r) => r.status === 404 });

  // BulkDeleteTags: the "delete these duplicate tags" use case — one of
  // the two entities ADR 0016 names for a real bulk-delete endpoint.
  const bulkId1 = `k6-http-bulk-tag-${__VU}-${__ITER}-${Date.now()}-1`;
  const bulkId2 = `k6-http-bulk-tag-${__VU}-${__ITER}-${Date.now()}-2`;
  const bulkId3 = `k6-http-bulk-tag-${__VU}-${__ITER}-${Date.now()}-3`;

  for (const bulkId of [bulkId1, bulkId2, bulkId3]) {
    res = invoke(`${SERVICE}/CreateTag`, JSON.stringify({ tag: { id: bulkId, key: 'genre', value: 'gonzo', scope: 'TAG_SCOPE_METADATA' } }), HEADERS);
    check(res, { 'setup: CreateTag status is 200': (r) => r.status === 200 });
  }

  res = invoke(`${SERVICE}/BulkDeleteTags`, JSON.stringify({ ids: [bulkId1, bulkId2] }), HEADERS);
  check(res, { 'BulkDeleteTags status is 200': (r) => r.status === 200 });

  res = invoke(`${SERVICE}/GetTag`, JSON.stringify({ id: bulkId1 }), HEADERS);
  check(res, { 'GetTag for bulkId1 after BulkDeleteTags is 404': (r) => r.status === 404 });
  res = invoke(`${SERVICE}/GetTag`, JSON.stringify({ id: bulkId2 }), HEADERS);
  check(res, { 'GetTag for bulkId2 after BulkDeleteTags is 404': (r) => r.status === 404 });
  res = invoke(`${SERVICE}/GetTag`, JSON.stringify({ id: bulkId3 }), HEADERS);
  check(res, { 'GetTag for bulkId3 (not in the batch) still exists': (r) => r.status === 200 });

  // All-or-nothing: a batch with one missing id must fail entirely — the
  // still-existing bulkId3 must not be removed either.
  res = invoke(`${SERVICE}/BulkDeleteTags`, JSON.stringify({ ids: [bulkId3, 'k6-http-tag-missing'] }), HEADERS);
  check(res, { 'BulkDeleteTags with a missing id is 404': (r) => r.status === 404 });

  res = invoke(`${SERVICE}/GetTag`, JSON.stringify({ id: bulkId3 }), HEADERS);
  check(res, { 'GetTag for bulkId3 after failed batch still exists (rolled back)': (r) => r.status === 200 });

  res = invoke(`${SERVICE}/DeleteTag`, JSON.stringify({ id: bulkId3 }), HEADERS);
  check(res, { 'cleanup: DeleteTag status is 200': (r) => r.status === 200 });
};
