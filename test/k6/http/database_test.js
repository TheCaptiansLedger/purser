// k6 HTTP/JSON suite for DatabaseService. See test/k6/http/job_test.js's
// header comment for the precedent this follows.
//
// Only GetDatabaseInfo is exercised here. Backup and Restore are both
// streaming RPCs, and Connect's HTTP/JSON transport for a streaming RPC
// is an enveloped binary wire format (a 1-byte flag + a 4-byte length
// prefix per frame) — not plain JSON, and not something k6's http module
// parses; Backup's coverage on this transport is skipped for that reason
// alone and picked up instead by test/k6/grpc/database_test.js.
//
// Restore additionally gets no k6 test in *either* protocol: a successful
// call is irreversibly destructive to the entire database this shared
// server is using, and makes the server process exit shortly after
// replying. See test/k6/grpc/database_test.js's header comment for the
// full reasoning and where Restore's actual behavior is covered instead.
import http from 'k6/http';
import { check } from 'k6';
import { options } from '../lib/options.js';
export { options };

const BASE_URL = __ENV.PURSER_HTTP_URL || 'http://localhost:7474';
const SERVICE = `${BASE_URL}/purser.database.v1.DatabaseService`;
const HEADERS = { headers: { 'Content-Type': 'application/json' } };

function invoke(url, body, headers) {
  const res = http.post(url, body, headers);
  console.log(JSON.stringify({ method: url, request: JSON.parse(body), response: res.json() }, null, 2));
  return res;
}

export default () => {
  const res = invoke(`${SERVICE}/GetDatabaseInfo`, JSON.stringify({}), HEADERS);
  check(res, {
    'GetDatabaseInfo status is 200': (r) => r.status === 200,
    'GetDatabaseInfo returns a driver': (r) => !!r.json('driver'),
    // collectionCounts is a proto map field: protojson omits it entirely
    // (rather than emitting {}) when the database has no documents in
    // any collection, so this only checks the response decoded as JSON
    // at all, not that the key is present — an empty database is a
    // legitimate state for this suite to run against.
    'GetDatabaseInfo returns a decodable JSON body': (r) => r.json() !== null,
  });
};
