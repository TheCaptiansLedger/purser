// k6 flow (HTTP/JSON twin of create_studio_test.js): "add a studio" — the
// task a UI/admin tool runs when registering a new studio. Since a
// Studio's parent_id must point at a Network, this flow registers the
// network first, exactly as a real "add studio" form would require you to
// pick or create the parent network before the studio itself can exist.
//
// Same fixture data as create_studio_test.js — fictionalized but
// shape-accurate, pulled from live StashDB studio responses during
// design. See that script's header for the sourcing note.
import http from 'k6/http';
import { check, fail } from 'k6';

const BASE_URL = __ENV.PURSER_HTTP_URL || 'http://localhost:7474';
const LIBRARY_ENTRY = `${BASE_URL}/purser.domain.v1.LibraryEntryService`;
const TAG = `${BASE_URL}/purser.domain.v1.TagService`;
const TAG_ASSIGNMENT = `${BASE_URL}/purser.domain.v1.TagAssignmentService`;
const HEADERS = { headers: { 'Content-Type': 'application/json' } };

function invoke(url, body, label) {
  const res = http.post(url, JSON.stringify(body), HEADERS);
  const ok = check(res, { [`${label} status is 200`]: (r) => r.status === 200 });
  if (!ok) {
    fail(`${label} failed: ${res.status} ${res.body}`);
  }
  console.log(JSON.stringify({
    method: url,
    request: body,
    response: res.json(),
  }, null, 2))
  return res;
}

export default () => {
  const suffix = `${__VU}-${__ITER}-${Date.now()}`;

  // ids are server-generated (docs/adr/0020-server-generated-kernel-entity-ids.md)
  // — never sent on Create, always read back from the response.
  const createdNetwork = invoke(
    `${LIBRARY_ENTRY}/CreateLibraryEntry`,
    {
      libraryEntry: {
        contentType: 'adult',
        kind: 'network',
        name: 'Twilight Media (Network)',
        overview: 'A boutique adult content network of independently-branded studios.',
        monitorMode: 'MONITOR_MODE_ALL',
      },
    },
    'CreateLibraryEntry(network)'
  );
  const networkId = createdNetwork.json('libraryEntry.id');

  const createdStudio = invoke(
    `${LIBRARY_ENTRY}/CreateLibraryEntry`,
    {
      libraryEntry: {
        contentType: 'adult',
        kind: 'studio',
        parentId: networkId,
        name: 'Twilight Media x Velvet Hour',
        overview: 'Premium feature-length scenes, launched under the Twilight Media umbrella.',
        monitorMode: 'MONITOR_MODE_ALL',
      },
    },
    'CreateLibraryEntry(studio)'
  );
  const studioId = createdStudio.json('libraryEntry.id');

  // value is suffixed per-VU: CreateTag is get-or-create on (scope, key,
  // value) (docs/adr/0019), so a literal value would make concurrent VUs
  // share one tag and race on this flow's own teardown DeleteTag.
  const createdNetworkTag = invoke(
    `${TAG}/CreateTag`,
    { tag: { key: 'network_type', value: `Boutique ${suffix}`, scope: 'TAG_SCOPE_USER', category: 'Misc' } },
    'CreateTag(network)'
  );
  const networkTagId = createdNetworkTag.json('tag.id');
  invoke(
    `${TAG_ASSIGNMENT}/CreateTagAssignment`,
    { tagAssignment: { tagId: networkTagId, entityType: 'ENTITY_TYPE_LIBRARY_ENTRY', entityId: networkId } },
    'CreateTagAssignment(network)'
  );

  const createdStudioTag = invoke(
    `${TAG}/CreateTag`,
    { tag: { key: 'studio_type', value: `Premium ${suffix}`, scope: 'TAG_SCOPE_USER', category: 'Misc' } },
    'CreateTag(studio)'
  );
  const studioTagId = createdStudioTag.json('tag.id');
  invoke(
    `${TAG_ASSIGNMENT}/CreateTagAssignment`,
    { tagAssignment: { tagId: studioTagId, entityType: 'ENTITY_TYPE_LIBRARY_ENTRY', entityId: studioId } },
    'CreateTagAssignment(studio)'
  );

  let res = invoke(`${LIBRARY_ENTRY}/ListLibraryEntries`, { kind: 'network', pageSize: 50 }, 'ListLibraryEntries(networks)');
  check(res, { 'list networks includes the created network': (r) => (r.json('libraryEntries') || []).some((e) => e.id === networkId) });

  res = invoke(`${LIBRARY_ENTRY}/ListLibraryEntries`, { kind: 'studio', parentId: networkId, pageSize: 50 }, 'ListLibraryEntries(studios)');
  check(res, { 'list studios under network includes the created studio': (r) => (r.json('libraryEntries') || []).some((e) => e.id === studioId) });

  // Tags are never embedded on LibraryEntry — TagAssignment is a separate
  // polymorphic join. Prove each association round-trips in both
  // directions: "this network's/studio's tags" and "everything tagged
  // this tag."
  res = invoke(`${TAG_ASSIGNMENT}/ListTagAssignments`, { entityType: 'ENTITY_TYPE_LIBRARY_ENTRY', entityId: networkId, pageSize: 10 }, 'ListTagAssignments(by network)');
  check(res, { "the network's tags include the created tag": (r) => (r.json('tagAssignments') || []).some((ta) => ta.tagId === networkTagId) });

  res = invoke(`${TAG_ASSIGNMENT}/ListTagAssignments`, { tagId: networkTagId, pageSize: 10 }, 'ListTagAssignments(by network tag)');
  check(res, { 'everything tagged the network tag includes the created network': (r) => (r.json('tagAssignments') || []).some((ta) => ta.entityId === networkId) });

  res = invoke(`${TAG_ASSIGNMENT}/ListTagAssignments`, { entityType: 'ENTITY_TYPE_LIBRARY_ENTRY', entityId: studioId, pageSize: 10 }, 'ListTagAssignments(by studio)');
  check(res, { "the studio's tags include the created tag": (r) => (r.json('tagAssignments') || []).some((ta) => ta.tagId === studioTagId) });

  res = invoke(`${TAG_ASSIGNMENT}/ListTagAssignments`, { tagId: studioTagId, pageSize: 10 }, 'ListTagAssignments(by studio tag)');
  check(res, { 'everything tagged the studio tag includes the created studio': (r) => (r.json('tagAssignments') || []).some((ta) => ta.entityId === studioId) });

  // Teardown, reverse order.
  invoke(
    `${TAG_ASSIGNMENT}/DeleteTagAssignment`,
    { tagId: studioTagId, entityType: 'ENTITY_TYPE_LIBRARY_ENTRY', entityId: studioId },
    'DeleteTagAssignment(studio)'
  );
  invoke(`${TAG}/DeleteTag`, { id: studioTagId }, 'DeleteTag(studio)');
  invoke(
    `${TAG_ASSIGNMENT}/DeleteTagAssignment`,
    { tagId: networkTagId, entityType: 'ENTITY_TYPE_LIBRARY_ENTRY', entityId: networkId },
    'DeleteTagAssignment(network)'
  );
  invoke(`${TAG}/DeleteTag`, { id: networkTagId }, 'DeleteTag(network)');
  invoke(`${LIBRARY_ENTRY}/DeleteLibraryEntry`, { id: studioId }, 'DeleteLibraryEntry(studio)');
  invoke(`${LIBRARY_ENTRY}/DeleteLibraryEntry`, { id: networkId }, 'DeleteLibraryEntry(network)');
};
