// k6 flow (HTTP/JSON twin of create_performer_test.js): "add a performer"
// — the task a UI/admin tool runs when registering a new performer that
// isn't in the library yet. One person, one profile, one attribute tag,
// then confirms the performer shows up in AfterDark's performer catalog.
//
// Same fixture data as create_performer_test.js — fictionalized but
// shape-accurate, pulled from live StashDB/ThePornDB performer responses
// during design. See that script's header for the sourcing note.
import http from 'k6/http';
import { check, fail } from 'k6';

const BASE_URL = __ENV.PURSER_HTTP_URL || 'http://localhost:7474';
const PERSON = `${BASE_URL}/purser.domain.v1.PersonService`;
const TAG = `${BASE_URL}/purser.domain.v1.TagService`;
const TAG_ASSIGNMENT = `${BASE_URL}/purser.domain.v1.TagAssignmentService`;
const PERFORMER_PROFILE = `${BASE_URL}/purser.afterdark.v1.PerformerProfileService`;
const BROWSE = `${BASE_URL}/purser.afterdark.v1.BrowseService`;
const HEADERS = { headers: { 'Content-Type': 'application/json' } };

function invoke(url, body, label) {
  const res = http.post(url, JSON.stringify(body), HEADERS);
  const ok = check(res, { [`${label} status is 200`]: (r) => r.status === 200 });
  if (!ok) {
    fail(`${label} failed: ${res.status} ${res.body}`);
  }
  return res;
}

export default () => {
  const suffix = `${__VU}-${__ITER}-${Date.now()}`;
  const personId = `k6-flow-http-performer-person-${suffix}`;
  const tagId = `k6-flow-http-performer-tag-${suffix}`;

  invoke(
    `${PERSON}/CreatePerson`,
    {
      person: {
        id: personId,
        name: 'Harlow Vance',
        sortName: 'Vance, Harlow',
        aliases: ['Harlow V.'],
        gender: 'GENDER_FEMALE',
        pronouns: 'she/her',
        nationality: 'American',
        overview: 'Performer active since 2021, primarily feature and boutique-studio scenes.',
        monitorMode: 'MONITOR_MODE_NONE',
      },
    },
    'CreatePerson'
  );

  invoke(
    `${PERFORMER_PROFILE}/CreatePerformerProfile`,
    {
      performerProfile: {
        personId: personId,
        cupSize: 'C',
        bandSize: '34',
        breastType: 'NATURAL',
        tattoos: [{ location: 'Left forearm', description: 'Floral sleeve piece' }],
        careerStartYear: 2021,
      },
    },
    'CreatePerformerProfile'
  );

  invoke(`${TAG}/CreateTag`, { tag: { id: tagId, key: 'attribute', value: 'Tattoos', scope: 'TAG_SCOPE_USER', category: 'People' } }, 'CreateTag');
  invoke(
    `${TAG_ASSIGNMENT}/CreateTagAssignment`,
    { tagAssignment: { tagId: tagId, entityType: 'ENTITY_TYPE_PERSON', entityId: personId } },
    'CreateTagAssignment(performer)'
  );

  let res = invoke(`${PERSON}/GetPerson`, { id: personId }, 'GetPerson');
  check(res, { 'GetPerson returns the created name': (r) => r.json('person.name') === 'Harlow Vance' });

  res = invoke(`${PERFORMER_PROFILE}/GetPerformerProfile`, { personId: personId }, 'GetPerformerProfile');
  check(res, { 'GetPerformerProfile returns the created cupSize': (r) => r.json('performerProfile.cupSize') === 'C' });

  res = invoke(`${BROWSE}/ListPerformers`, { pageSize: 50 }, 'ListPerformers');
  check(res, { 'ListPerformers includes the created performer': (r) => (r.json('performers') || []).some((p) => p.person && p.person.id === personId) });

  // Tags are never embedded on Person/PerformerView — TagAssignment is a
  // separate polymorphic join, same as ExternalID/Image. Prove the
  // association actually round-trips in both directions: "this
  // performer's tags" and "everything tagged this tag."
  res = invoke(`${TAG_ASSIGNMENT}/ListTagAssignments`, { entityType: 'ENTITY_TYPE_PERSON', entityId: personId, pageSize: 10 }, 'ListTagAssignments(by performer)');
  check(res, { "the performer's tags include the created tag": (r) => (r.json('tagAssignments') || []).some((ta) => ta.tagId === tagId) });

  res = invoke(`${TAG_ASSIGNMENT}/ListTagAssignments`, { tagId: tagId, pageSize: 10 }, 'ListTagAssignments(by tag)');
  check(res, { 'everything tagged this tag includes the created performer': (r) => (r.json('tagAssignments') || []).some((ta) => ta.entityId === personId) });

  // Teardown, reverse order.
  invoke(`${TAG_ASSIGNMENT}/DeleteTagAssignment`, { tagId: tagId, entityType: 'ENTITY_TYPE_PERSON', entityId: personId }, 'DeleteTagAssignment');
  invoke(`${TAG}/DeleteTag`, { id: tagId }, 'DeleteTag');
  invoke(`${PERFORMER_PROFILE}/DeletePerformerProfile`, { personId: personId }, 'DeletePerformerProfile');
  invoke(`${PERSON}/DeletePerson`, { id: personId }, 'DeletePerson');
};
