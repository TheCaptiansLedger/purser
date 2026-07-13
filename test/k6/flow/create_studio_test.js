// k6 flow: "add a studio" — the task a UI/admin tool runs when
// registering a new studio. Since a Studio's parent_id must point at a
// Network (LibraryEntry self-referential hierarchy), this flow registers
// the network first, exactly as a real "add studio" form would require you
// to pick or create the parent network before the studio itself can exist.
//
// Fixture data is fictionalized but shape-accurate: pulled from live
// StashDB (queryStudios) during design, which confirmed the real
// Network -> Studio hierarchy this maps onto (e.g. "Adult Time (Network)"
// parenting "Adult Time x Girlfriends Films", and "Girlfriends Films"
// itself parenting "Bad Lesbian"/"Bus Stop"/"Cheer Squad Slumber Parties")
// — the naming pattern is real, the actual names here are invented.
import grpc from 'k6/net/grpc';
import { check, fail } from 'k6';

const ADDR = __ENV.PURSER_GRPC_ADDR || 'localhost:7474';

const client = new grpc.Client();
client.load(
  ['../../../proto'],
  'purser/domain/v1/library_entry.proto',
  'purser/domain/v1/tag.proto',
  'purser/domain/v1/tag_assignment.proto'
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
  const networkId = `k6-flow-studio-network-${suffix}`;
  const studioId = `k6-flow-studio-studio-${suffix}`;
  const networkTagId = `k6-flow-studio-network-tag-${suffix}`;
  const studioTagId = `k6-flow-studio-studio-tag-${suffix}`;

  invoke(
    'purser.domain.v1.LibraryEntryService/CreateLibraryEntry',
    {
      libraryEntry: {
        id: networkId,
        contentType: 'adult',
        kind: 'network',
        name: 'Twilight Media (Network)',
        overview: 'A boutique adult content network of independently-branded studios.',
        monitorMode: 'MONITOR_MODE_ALL',
      },
    },
    'CreateLibraryEntry(network)'
  );

  invoke(
    'purser.domain.v1.LibraryEntryService/CreateLibraryEntry',
    {
      libraryEntry: {
        id: studioId,
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

  invoke(
    'purser.domain.v1.TagService/CreateTag',
    { tag: { id: networkTagId, key: 'network_type', value: 'Boutique', scope: 'TAG_SCOPE_USER', category: 'Misc' } },
    'CreateTag(network)'
  );
  invoke(
    'purser.domain.v1.TagAssignmentService/CreateTagAssignment',
    { tagAssignment: { tagId: networkTagId, entityType: 'ENTITY_TYPE_LIBRARY_ENTRY', entityId: networkId } },
    'CreateTagAssignment(network)'
  );

  invoke(
    'purser.domain.v1.TagService/CreateTag',
    { tag: { id: studioTagId, key: 'studio_type', value: 'Premium', scope: 'TAG_SCOPE_USER', category: 'Misc' } },
    'CreateTag(studio)'
  );
  invoke(
    'purser.domain.v1.TagAssignmentService/CreateTagAssignment',
    { tagAssignment: { tagId: studioTagId, entityType: 'ENTITY_TYPE_LIBRARY_ENTRY', entityId: studioId } },
    'CreateTagAssignment(studio)'
  );

  const networks = invoke('purser.domain.v1.LibraryEntryService/ListLibraryEntries', { kind: 'network', pageSize: 50 }, 'ListLibraryEntries(networks)');
  check(networks, { 'list networks includes the created network': (m) => m && m.libraryEntries && m.libraryEntries.some((e) => e.id === networkId) });

  const studios = invoke(
    'purser.domain.v1.LibraryEntryService/ListLibraryEntries',
    { kind: 'studio', parentId: networkId, pageSize: 50 },
    'ListLibraryEntries(studios)'
  );
  check(studios, { 'list studios under network includes the created studio': (m) => m && m.libraryEntries && m.libraryEntries.some((e) => e.id === studioId) });

  // Tags are never embedded on LibraryEntry — TagAssignment is a separate
  // polymorphic join. Prove each association round-trips in both
  // directions: "this network's/studio's tags" and "everything tagged
  // this tag."
  const networkTags = invoke(
    'purser.domain.v1.TagAssignmentService/ListTagAssignments',
    { entityType: 'ENTITY_TYPE_LIBRARY_ENTRY', entityId: networkId, pageSize: 10 },
    'ListTagAssignments(by network)'
  );
  check(networkTags, {
    "the network's tags include the created tag": (m) => m && m.tagAssignments && m.tagAssignments.some((ta) => ta.tagId === networkTagId),
  });

  const taggedWithNetworkTag = invoke(
    'purser.domain.v1.TagAssignmentService/ListTagAssignments',
    { tagId: networkTagId, pageSize: 10 },
    'ListTagAssignments(by network tag)'
  );
  check(taggedWithNetworkTag, {
    'everything tagged the network tag includes the created network': (m) => m && m.tagAssignments && m.tagAssignments.some((ta) => ta.entityId === networkId),
  });

  const studioTags = invoke(
    'purser.domain.v1.TagAssignmentService/ListTagAssignments',
    { entityType: 'ENTITY_TYPE_LIBRARY_ENTRY', entityId: studioId, pageSize: 10 },
    'ListTagAssignments(by studio)'
  );
  check(studioTags, {
    "the studio's tags include the created tag": (m) => m && m.tagAssignments && m.tagAssignments.some((ta) => ta.tagId === studioTagId),
  });

  const taggedWithStudioTag = invoke(
    'purser.domain.v1.TagAssignmentService/ListTagAssignments',
    { tagId: studioTagId, pageSize: 10 },
    'ListTagAssignments(by studio tag)'
  );
  check(taggedWithStudioTag, {
    'everything tagged the studio tag includes the created studio': (m) => m && m.tagAssignments && m.tagAssignments.some((ta) => ta.entityId === studioId),
  });

  // Teardown, reverse order.
  invoke(
    'purser.domain.v1.TagAssignmentService/DeleteTagAssignment',
    { tagId: studioTagId, entityType: 'ENTITY_TYPE_LIBRARY_ENTRY', entityId: studioId },
    'DeleteTagAssignment(studio)'
  );
  invoke('purser.domain.v1.TagService/DeleteTag', { id: studioTagId }, 'DeleteTag(studio)');
  invoke(
    'purser.domain.v1.TagAssignmentService/DeleteTagAssignment',
    { tagId: networkTagId, entityType: 'ENTITY_TYPE_LIBRARY_ENTRY', entityId: networkId },
    'DeleteTagAssignment(network)'
  );
  invoke('purser.domain.v1.TagService/DeleteTag', { id: networkTagId }, 'DeleteTag(network)');
  invoke('purser.domain.v1.LibraryEntryService/DeleteLibraryEntry', { id: studioId }, 'DeleteLibraryEntry(studio)');
  invoke('purser.domain.v1.LibraryEntryService/DeleteLibraryEntry', { id: networkId }, 'DeleteLibraryEntry(network)');

  client.close();
};
