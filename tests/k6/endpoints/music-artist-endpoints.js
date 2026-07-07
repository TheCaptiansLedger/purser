// Endpoint tests for artist enrichment and release-group release listing.
//
// Functional: k6 run tests/k6/endpoints/music-artist-endpoints.js
// Load:       k6 run --vus 20 --duration 30s tests/k6/endpoints/music-artist-endpoints.js

import http from 'k6/http';
import { check, group } from 'k6';
import { BASE_URL, options, JSON_HEADERS } from '../config.js';

export { options };

// reoMBID is REO Speedwagon's canonical MusicBrainz identifier.
// Their metadata is stable and all enrichment fields are well-populated.
const reoMBID = 'bdc70372-7e8a-4cb9-8d33-f036b3b7cdc1';

function createTestArtist() {
  const res = http.post(`${BASE_URL}/library-entries`,
    JSON.stringify({ kind: 'artist', contentType: 'music', name: 'Test Artist' }),
    { headers: JSON_HEADERS });
  check(res, { 'create artist 201': r => r.status === 201 });
  return res.json('id');
}

function createTestGroup(entryId) {
  const res = http.post(`${BASE_URL}/groups`,
    JSON.stringify({ libraryEntryId: entryId, title: 'Hi Infidelity', year: 1980 }),
    { headers: JSON_HEADERS });
  check(res, { 'create group 201': r => r.status === 201 });
  return res.json('id');
}

export default function () {

  group('GET /metadata/entries/enrich — missing params', () => {
    const cases = [
      `${BASE_URL}/metadata/entries/enrich`,
      `${BASE_URL}/metadata/entries/enrich?source=mbz&contentType=music`,
      `${BASE_URL}/metadata/entries/enrich?source=mbz&externalId=${reoMBID}`,
      `${BASE_URL}/metadata/entries/enrich?externalId=${reoMBID}&contentType=music`,
    ];
    for (const url of cases) {
      const res = http.get(url);
      check(res, { [`missing params → 400: ${url}`]: r => r.status === 400 });
    }
  });

  group('GET /metadata/entries/enrich — unknown source', () => {
    const res = http.get(`${BASE_URL}/metadata/entries/enrich?source=bogus&externalId=${reoMBID}&contentType=music`);
    check(res, {
      'unknown source → 400': r => r.status === 400,
      'code is UNKNOWN_SOURCE': r => r.json('code') === 'UNKNOWN_SOURCE',
    });
  });

  group('GET /metadata/entries/enrich — not found', () => {
    const res = http.get(`${BASE_URL}/metadata/entries/enrich?source=mbz&externalId=aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa&contentType=music`);
    check(res, { 'nonexistent MBID → 404': r => r.status === 404 });
  });

  group('GET /metadata/entries/enrich — REO Speedwagon enrichment', () => {
    const res = http.get(`${BASE_URL}/metadata/entries/enrich?source=mbz&externalId=${reoMBID}&contentType=music`);
    check(res, {
      'enrich 200':                    r => r.status === 200,
      'artist_type == group':          r => r.json('artist_type') === 'group',
      'founded_date non-empty':        r => !!r.json('founded_date'),
      'aliases is non-empty array':    r => Array.isArray(r.json('aliases')) && r.json('aliases').length > 0,
      'official_url non-empty':        r => !!r.json('official_url'),
      'lastfm_url non-empty':          r => !!r.json('lastfm_url'),
      'born_date absent (group)':      r => r.json('born_date') === null || r.json('born_date') === undefined,
    });
  });

  group('GET /groups/{id}/releases — barcode and isDefault fields', () => {
    const entryId = createTestArtist();
    const groupId = createTestGroup(entryId);

    // Create 5 releases with distinct barcodes; the first is the default.
    const editionBarcode = '074646161425';
    for (let i = 0; i < 5; i++) {
      const releaseRes = http.post(`${BASE_URL}/music/releases`,
        JSON.stringify({
          groupId,
          libraryEntryId: entryId,
          title:    `Hi Infidelity Edition ${i + 1}`,
          barcode:  i === 0 ? editionBarcode : `barcode${i}`,
          isDefault: i === 0,
          status:   'stub',
        }),
        { headers: JSON_HEADERS });
      check(releaseRes, { [`create release ${i + 1} 201`]: r => r.status === 201 });
    }

    const listRes = http.get(`${BASE_URL}/groups/${groupId}/releases`);
    check(listRes, {
      'list 200':                           r => r.status === 200,
      'list: has 5 releases':               r => Array.isArray(r.json()) && r.json().length === 5,
      'list: expected barcode present':     r => r.json().some(rel => rel.barcode === editionBarcode),
      'list: exactly one isDefault=true':   r => r.json().filter(rel => rel.isDefault).length === 1,
      'list: isDefault release has barcode': r => {
        const def = r.json().find(rel => rel.isDefault);
        return def && def.barcode === editionBarcode;
      },
    });
  });
}
