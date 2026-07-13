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
  return res;
}

export default () => {
  const suffix = `${__VU}-${__ITER}-${Date.now()}`;
  const networkId = `k6-flow-http-studio-network-${suffix}`;
  const studioId = `k6-flow-http-studio-studio-${suffix}`;
  const networkTagId = `k6-flow-http-studio-network-tag-${suffix}`;
  const studioTagId = `k6-flow-http-studio-studio-tag-${suffix}`;

  invoke(
    `${LIBRARY_ENTRY}/CreateLibraryEntry`,
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
    `${LIBRARY_ENTRY}/CreateLibraryEntry`,
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
    `${TAG}/CreateTag`,
    { tag: { id: networkTagId, key: 'network_type', value: 'Boutique', scope: 'TAG_SCOPE_USER', category: 'Misc' } },
    'CreateTag(network)'
  );
  invoke(
    `${TAG_ASSIGNMENT}/CreateTagAssignment`,
    { tagAssignment: { tagId: networkTagId, entityType: 'ENTITY_TYPE_LIBRARY_ENTRY', entityId: networkId } },
    'CreateTagAssignment(network)'
  );

  invoke(
    `${TAG}/CreateTag`,
    { tag: { id: studioTagId, key: 'studio_type', value: 'Premium', scope: 'TAG_SCOPE_USER', category: 'Misc' } },
    'CreateTag(studio)'
  );
  invoke(
    `${TAG_ASSIGNMENT}/CreateTagAssignment`,
    { tagAssignment: { tagId: studioTagId, entityType: 'ENTITY_TYPE_LIBRARY_ENTRY', entityId: studioId } },
    'CreateTagAssignment(studio)'
  );

  let res = invoke(`${LIBRARY_ENTRY}/ListLibraryEntries`, { kind: 'network', pageSize: 50 }, 'ListLibraryEntries(networks)');
  check(res, { 'list networks includes the created network': (r) => (r.json('libraryEntries') || []).some((e) => e.id === networkId) });

  res = invoke(`${LIBRARY_ENTRY}/ListLibraryEntries`, { kind: 'studio', parentId: networkId, pageSize: 50 }, 'ListLibraryEntries(studios)');
  check(res, { 'list studios under network includes the created studio': (r) => (r.json('libraryEntries') || []).some((e) => e.id === studioId) });

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
