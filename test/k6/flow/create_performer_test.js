// k6 flow: "add a performer" — the task a UI/admin tool runs when
// registering a new performer that isn't in the library yet. One person,
// one profile, one attribute tag, then confirms the performer shows up in
// AfterDark's performer catalog.
//
// Fixture data is fictionalized but shape-accurate: pulled from live
// StashDB (queryPerformers) and ThePornDB (/performers) responses during
// design — real field names (gender/ethnicity/eye_color/hair_color/height/
// breast_type/career_start_year on StashDB; measurements/cupsize/tattoos/
// piercings under ThePornDB's extras) and realistic value ranges, with the
// actual name/attributes invented rather than reused from any real record.
import grpc from 'k6/net/grpc';
import { check, fail } from 'k6';

const ADDR = __ENV.PURSER_GRPC_ADDR || 'localhost:7474';

const client = new grpc.Client();
client.load(
  ['../../../proto'],
  'purser/domain/v1/person.proto',
  'purser/domain/v1/tag.proto',
  'purser/domain/v1/tag_assignment.proto',
  'purser/afterdark/v1/performer_profile.proto',
  'purser/afterdark/v1/browse.proto'
);

function invoke(method, request, label) {
  const res = client.invoke(method, request);
  const ok = check(res, { [`${label} status is OK`]: (r) => r && r.status === grpc.StatusOK });
  if (!ok) {
    fail(`${label} failed: ${res && res.status} ${res && res.error && res.error.message}`);
  }
  console.log(JSON.stringify({
    method: method,
    request: request,
    response: res.message,
  }, null, 2))
  return res.message;
}

export default () => {
  client.connect(ADDR, { plaintext: true });

  const suffix = `${__VU}-${__ITER}-${Date.now()}`;

  // ids are server-generated (docs/adr/0020-server-generated-kernel-entity-ids.md)
  // — never sent on Create, always read back from the response.
  //
  // A fictional performer, shaped like a real StashDB performer record
  // (name/aliases/gender/pronouns/nationality) plus a real
  // afterdark.PerformerProfile record (cup/band size, breast type, a
  // tattoo, career start year) — none of it drawn from an actual person.
  const createdPerson = invoke(
    'purser.domain.v1.PersonService/CreatePerson',
    {
      person: {
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
  const personId = createdPerson.person.id;

  invoke(
    'purser.afterdark.v1.PerformerProfileService/CreatePerformerProfile',
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

  // value is suffixed per-VU: CreateTag is get-or-create on (scope, key,
  // value) (docs/adr/0019), so a literal value would make concurrent VUs
  // share one tag and race on this flow's own teardown DeleteTag.
  const createdTag = invoke(
    'purser.domain.v1.TagService/CreateTag',
    { tag: { key: 'attribute', value: `Tattoos ${suffix}`, scope: 'TAG_SCOPE_USER', category: 'People' } },
    'CreateTag'
  );
  const tagId = createdTag.tag.id;
  invoke(
    'purser.domain.v1.TagAssignmentService/CreateTagAssignment',
    { tagAssignment: { tagId: tagId, entityType: 'ENTITY_TYPE_PERSON', entityId: personId } },
    'CreateTagAssignment(performer)'
  );

  const person = invoke('purser.domain.v1.PersonService/GetPerson', { id: personId }, 'GetPerson');
  check(person, { 'GetPerson returns the created name': (p) => p && p.person && p.person.name === 'Harlow Vance' });

  const profile = invoke('purser.afterdark.v1.PerformerProfileService/GetPerformerProfile', { personId: personId }, 'GetPerformerProfile');
  check(profile, { 'GetPerformerProfile returns the created cupSize': (p) => p && p.performerProfile && p.performerProfile.cupSize === 'C' });

  const performers = invoke('purser.afterdark.v1.BrowseService/ListPerformers', { pageSize: 50 }, 'ListPerformers');
  check(performers, {
    'ListPerformers includes the created performer': (m) => m && m.performers && m.performers.some((p) => p.person && p.person.id === personId),
  });

  // Tags are never embedded on Person/PerformerView — TagAssignment is a
  // separate polymorphic join, same as ExternalID/Image. Prove the
  // association actually round-trips in both directions: "this
  // performer's tags" and "everything tagged this tag."
  const performerTags = invoke(
    'purser.domain.v1.TagAssignmentService/ListTagAssignments',
    { entityType: 'ENTITY_TYPE_PERSON', entityId: personId, pageSize: 10 },
    'ListTagAssignments(by performer)'
  );
  check(performerTags, {
    "the performer's tags include the created tag": (m) => m && m.tagAssignments && m.tagAssignments.some((ta) => ta.tagId === tagId),
  });

  const taggedEntities = invoke(
    'purser.domain.v1.TagAssignmentService/ListTagAssignments',
    { tagId: tagId, pageSize: 10 },
    'ListTagAssignments(by tag)'
  );
  check(taggedEntities, {
    'everything tagged this tag includes the created performer': (m) => m && m.tagAssignments && m.tagAssignments.some((ta) => ta.entityId === personId),
  });

  // Teardown, reverse order.
  invoke(
    'purser.domain.v1.TagAssignmentService/DeleteTagAssignment',
    { tagId: tagId, entityType: 'ENTITY_TYPE_PERSON', entityId: personId },
    'DeleteTagAssignment'
  );
  invoke('purser.domain.v1.TagService/DeleteTag', { id: tagId }, 'DeleteTag');
  invoke('purser.afterdark.v1.PerformerProfileService/DeletePerformerProfile', { personId: personId }, 'DeletePerformerProfile');
  invoke('purser.domain.v1.PersonService/DeletePerson', { id: personId }, 'DeletePerson');

  client.close();
};
