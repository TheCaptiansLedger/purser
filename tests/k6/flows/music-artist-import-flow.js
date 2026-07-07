// End-to-end artist enrichment flow — verifies that the enrichment endpoint
// returns all expected metadata keys for known artists.
//
// REO Speedwagon (group): artist_type=group, founded_date, founded_location,
//   aliases, isni, official_url, lastfm_url.
//
// Stevie Nicks (solo/Person): artist_type=person, born_date, born_location.
//
// Functional: k6 run tests/k6/flows/music-artist-import-flow.js
// Load:       k6 run --vus 10 --duration 60s tests/k6/flows/music-artist-import-flow.js

import http from 'k6/http';
import { check, group } from 'k6';
import { BASE_URL, options } from '../config.js';

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
}
