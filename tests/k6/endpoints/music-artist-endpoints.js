// Endpoint tests for GET /api/v1/metadata/entries/enrich (artist enrichment).
//
// Functional: k6 run tests/k6/endpoints/music-artist-endpoints.js
// Load:       k6 run --vus 20 --duration 30s tests/k6/endpoints/music-artist-endpoints.js

import http from 'k6/http';
import { check, group } from 'k6';
import { BASE_URL, options } from '../config.js';

export { options };

// reoMBID is REO Speedwagon's canonical MusicBrainz identifier.
// Their metadata is stable and all enrichment fields are well-populated.
const reoMBID = 'bdc70372-7e8a-4cb9-8d33-f036b3b7cdc1';

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
}
