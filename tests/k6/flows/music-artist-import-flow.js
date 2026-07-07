// End-to-end artist enrichment flow — verifies that the enrichment endpoint
// returns all expected metadata keys for known artists.
//
// REO Speedwagon (group): artist_type=group, founded_date, founded_location,
//   aliases, isni, official_url, lastfm_url.
//
// Stevie Nicks (solo/Person): artist_type=person, born_date, born_location.
//
// Also verifies that the releases-by-group endpoint returns barcode and
// isDefault fields correctly when releases are created directly.
//
// Functional: k6 run tests/k6/flows/music-artist-import-flow.js
// Load:       k6 run --vus 10 --duration 60s tests/k6/flows/music-artist-import-flow.js

import http from 'k6/http';
import { check, group } from 'k6';
import { BASE_URL, options, JSON_HEADERS } from '../config.js';

export { options };

const reoMBID         = 'bdc70372-7e8a-4cb9-8d33-f036b3b7cdc1';
const stevieNicksMBID = 'b7f2cca2-72c6-41fb-ae33-53370fc62fe7';

export default function () {

  group('artist enrichment: all metadata keys present after import', () => {

    group('REO Speedwagon — group artist keys', () => {
      const res = http.get(
        `${BASE_URL}/metadata/entries/enrich?source=mbz&externalId=${reoMBID}&contentType=music`
      );
      check(res, {
        'enrich 200':                       r => r.status === 200,
        'artist_type == group':             r => r.json('artist_type') === 'group',
        'founded_date is non-empty string': r => typeof r.json('founded_date') === 'string' && r.json('founded_date').length > 0,
        'founded_location non-empty':       r => !!r.json('founded_location'),
        'aliases is non-empty array':       r => Array.isArray(r.json('aliases')) && r.json('aliases').length > 0,
        'isni non-empty':                   r => !!r.json('isni'),
        'official_url non-empty':           r => !!r.json('official_url'),
        'lastfm_url non-empty':             r => !!r.json('lastfm_url'),
        'born_date absent for group':       r => r.json('born_date') === null || r.json('born_date') === undefined,
      });
    });

    group('Stevie Nicks — solo artist (Person) keys', () => {
      const res = http.get(
        `${BASE_URL}/metadata/entries/enrich?source=mbz&externalId=${stevieNicksMBID}&contentType=music`
      );
      check(res, {
        'enrich 200':                       r => r.status === 200,
        'artist_type == person':            r => r.json('artist_type') === 'person',
        'born_date is non-empty string':    r => typeof r.json('born_date') === 'string' && r.json('born_date').length > 0,
        'born_location non-empty':          r => !!r.json('born_location'),
        'founded_date absent for person':   r => r.json('founded_date') === null || r.json('founded_date') === undefined,
        'dissolved_date absent for person': r => r.json('dissolved_date') === null || r.json('dissolved_date') === undefined,
      });
    });

  });

  group('barcode lookup: releases endpoint returns barcode and isDefault fields', () => {
    // Create a group with multiple releases, one carrying the Hi Infidelity
    // 2024 Digital barcode. Verifies the releases-by-group endpoint correctly
    // round-trips barcode and isDefault, decoupled from the import pipeline.
    //
    // Full pipeline verification (import REO Speedwagon → FetchReleaseGroupReleases
    // populates releases automatically) requires Task 15 (#407).
    const entryRes = http.post(`${BASE_URL}/library-entries`,
      JSON.stringify({ kind: 'artist', contentType: 'music', name: 'REO Speedwagon' }),
      { headers: JSON_HEADERS });
    check(entryRes, { 'create entry 201': r => r.status === 201 });
    const entryId = entryRes.json('id');

    const groupRes = http.post(`${BASE_URL}/groups`,
      JSON.stringify({ libraryEntryId: entryId, title: 'Hi Infidelity', year: 1980 }),
      { headers: JSON_HEADERS });
    check(groupRes, { 'create group 201': r => r.status === 201 });
    const groupId = groupRes.json('id');

    const targetBarcode = '074646161425';
    const editionCount  = 5;
    for (let i = 0; i < editionCount; i++) {
      const rr = http.post(`${BASE_URL}/music/releases`,
        JSON.stringify({
          groupId,
          libraryEntryId: entryId,
          title:    `Hi Infidelity Edition ${i + 1}`,
          barcode:  i === 0 ? targetBarcode : `0000000000${i}`,
          isDefault: i === 0,
          status:   'stub',
        }),
        { headers: JSON_HEADERS });
      check(rr, { [`create release ${i + 1} 201`]: r => r.status === 201 });
    }

    const listRes = http.get(`${BASE_URL}/groups/${groupId}/releases`);
    check(listRes, {
      'list 200':                           r => r.status === 200,
      'list: has 5 releases':               r => r.json().length === editionCount,
      'list: barcode 074646161425 present': r => r.json().some(rel => rel.barcode === targetBarcode),
      'list: exactly one isDefault=true':   r => r.json().filter(rel => rel.isDefault).length === 1,
    });
  });
}
