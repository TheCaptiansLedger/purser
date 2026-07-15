// k6 HTTP/JSON suite for TagService. See test/k6/http/person_test.js for
// the pattern this follows.
import http from 'k6/http';
import { check } from 'k6';
import { options } from '../lib/options.js';
export { options };

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
  // id is server-generated (docs/adr/0020-server-generated-kernel-entity-ids.md)
  // — never sent on Create, always read back from the response. value is
  // still suffixed per-VU/iteration: Tag identity is (scope, key, value)
  // (docs/adr/0019-tag-identity-and-get-or-create.md), so a shared value
  // would get-or-create-collide across concurrent VUs.
  const value = `gonzo-${__VU}-${__ITER}-${Date.now()}`;
  const entityId = 'k6-tag-deletion-entity-1';

  let res = invoke(`${SERVICE}/CreateTag`, JSON.stringify({ tag: { key: 'genre', value: value, scope: 'TAG_SCOPE_METADATA' } }), HEADERS);
  check(res, {
    'CreateTag status is 200': (r) => r.status === 200,
    'CreateTag returns an id': (r) => !!r.json('tag.id'),
  });
  const id = res.json('tag.id');

  res = invoke(`${SERVICE}/GetTag`, JSON.stringify({ id: id }), HEADERS);
  check(res, {
    'GetTag status is 200': (r) => r.status === 200,
    'GetTag returns the created value': (r) => r.json('tag.value') === value,
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
  // the two entities ADR 0016 names for a real bulk-delete endpoint. Each
  // gets its own value — Tag identity is (scope, key, value), so three
  // creates with the same value would get-or-create-collide into one row
  // instead of three (docs/adr/0019-tag-identity-and-get-or-create.md).
  const bulkIds = [];
  for (let i = 0; i < 3; i++) {
    res = invoke(
      `${SERVICE}/CreateTag`,
      JSON.stringify({ tag: { key: 'genre', value: `gonzo-bulk-${__VU}-${__ITER}-${Date.now()}-${i}`, scope: 'TAG_SCOPE_METADATA' } }),
      HEADERS
    );
    check(res, {
      'setup: CreateTag status is 200': (r) => r.status === 200,
      'setup: CreateTag returns an id': (r) => !!r.json('tag.id'),
    });
    bulkIds.push(res.json('tag.id'));
  }
  const [bulkId1, bulkId2, bulkId3] = bulkIds;

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
