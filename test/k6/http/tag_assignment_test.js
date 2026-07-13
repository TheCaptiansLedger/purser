// k6 HTTP/JSON suite for TagAssignmentService — a join-shaped, polymorphic
// entity keyed by the composite (tagId, entityType, entityId). No Update:
// nothing about a TagAssignment is mutable. See
// test/k6/http/external_id_test.js for the entityType convention this
// follows.
import http from 'k6/http';
import { check } from 'k6';

const BASE_URL = __ENV.PURSER_HTTP_URL || 'http://localhost:7474';
const SERVICE = `${BASE_URL}/purser.domain.v1.TagAssignmentService`;
const HEADERS = { headers: { 'Content-Type': 'application/json' } };

export default () => {
  const tagId = `k6-http-tag-${__VU}-${__ITER}-${Date.now()}`;
  const entityId = 'k6-person-1';
  const entityType = 'ENTITY_TYPE_PERSON';

  let res = http.post(
    `${SERVICE}/CreateTagAssignment`,
    JSON.stringify({ tagAssignment: { tagId: tagId, entityType: entityType, entityId: entityId } }),
    HEADERS
  );
  check(res, {
    'CreateTagAssignment status is 200': (r) => r.status === 200,
    'CreateTagAssignment returns the tagId': (r) => r.json('tagAssignment.tagId') === tagId,
  });

  res = http.post(`${SERVICE}/GetTagAssignment`, JSON.stringify({ tagId: tagId, entityType: entityType, entityId: entityId }), HEADERS);
  check(res, {
    'GetTagAssignment status is 200': (r) => r.status === 200,
    'GetTagAssignment returns the created entityId': (r) => r.json('tagAssignment.entityId') === entityId,
  });

  res = http.post(`${SERVICE}/ListTagAssignments`, JSON.stringify({ tagId: tagId, pageSize: 10 }), HEADERS);
  check(res, {
    'ListTagAssignments filtered by tagId status is 200': (r) => r.status === 200,
    'ListTagAssignments filtered by tagId includes the created assignment': (r) =>
      (r.json('tagAssignments') || []).some((ta) => ta.entityId === entityId),
  });

  res = http.post(`${SERVICE}/ListTagAssignments`, JSON.stringify({ entityType: entityType, entityId: entityId, pageSize: 10 }), HEADERS);
  check(res, {
    'ListTagAssignments filtered by entity status is 200': (r) => r.status === 200,
    'ListTagAssignments filtered by entity includes the created assignment': (r) => (r.json('tagAssignments') || []).some((ta) => ta.tagId === tagId),
  });

  res = http.post(`${SERVICE}/DeleteTagAssignment`, JSON.stringify({ tagId: tagId, entityType: entityType, entityId: entityId }), HEADERS);
  check(res, { 'DeleteTagAssignment status is 200': (r) => r.status === 200 });

  res = http.post(`${SERVICE}/GetTagAssignment`, JSON.stringify({ tagId: tagId, entityType: entityType, entityId: entityId }), HEADERS);
  check(res, { 'GetTagAssignment after Delete is 404 (NotFound)': (r) => r.status === 404 });
};
