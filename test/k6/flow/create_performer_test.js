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
  return res.message;
}

export default () => {
  client.connect(ADDR, { plaintext: true });

  const suffix = `${__VU}-${__ITER}-${Date.now()}`;
  const personId = `k6-flow-performer-person-${suffix}`;
  const tagId = `k6-flow-performer-tag-${suffix}`;

  // A fictional performer, shaped like a real StashDB performer record
  // (name/aliases/gender/pronouns/nationality) plus a real
  // afterdark.PerformerProfile record (cup/band size, breast type, a
  // tattoo, career start year) — none of it drawn from an actual person.
  invoke(
    'purser.domain.v1.PersonService/CreatePerson',
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

  invoke(
    'purser.domain.v1.TagService/CreateTag',
    { tag: { id: tagId, key: 'attribute', value: 'Tattoos', scope: 'TAG_SCOPE_USER', category: 'People' } },
    'CreateTag'
  );
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
