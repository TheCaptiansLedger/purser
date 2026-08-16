// k6 gRPC suite for DatabaseService. See test/k6/grpc/job_test.js for the
// pattern this follows, notably its server-streaming WatchJob case, which
// Backup below mirrors. See docs/technical/database-backup-restore.md.
//
// Restore is deliberately not exercised here, or in
// test/k6/http/database_test.js — a successful Restore is irreversibly
// destructive to the *entire* database this shared server is using, and
// makes the server process exit shortly after replying so an external
// supervisor restarts it. Running it from a k6 suite would wipe every
// other suite's fixtures and then kill the server mid-run. Restore's
// actual behavior (validation rejects bad input, clear+replay round-trips
// real data, the process-exit callback fires only on success) is instead
// covered by internal/adapters/database/{badger,sql}'s Go tests and
// internal/api/connect/database_test.go, which freely create and destroy
// throwaway datastore instances per test case — more rigorous coverage
// than a shared k6 server could safely provide anyway.
import grpc from 'k6/net/grpc';
import encoding from 'k6/encoding';
import { check } from 'k6';
import { options } from '../lib/options.js';
export { options };

const ADDR = __ENV.PURSER_GRPC_ADDR || 'localhost:7474';

const client = new grpc.Client();
client.load(['../../../proto'], 'purser/database/v1/database.proto');

function invoke(method, request) {
  const res = client.invoke(method, request);
  console.log(JSON.stringify({ method: method, request: request, response: res.message }, null, 2));
  return res;
}

export default () => {
  client.connect(ADDR, { plaintext: true });

  const res = invoke('purser.database.v1.DatabaseService/GetDatabaseInfo', {});
  check(res, {
    'GetDatabaseInfo status is OK': (r) => r && r.status === grpc.StatusOK,
    'GetDatabaseInfo returns a driver': (r) => r && r.message && !!r.message.driver,
    'GetDatabaseInfo returns collection_counts': (r) => r && r.message && !!r.message.collectionCounts,
  });

  // Backup (server-streaming): collect every chunk until the stream ends
  // on its own, reassemble the artifact, and confirm it's the JSONL
  // format docs/technical/database-backup-restore.md defines, starting
  // with the version header line. Read-only and non-destructive, so safe
  // to run against a shared server (unlike Restore, see above).
  const stream = new grpc.Stream(client, 'purser.database.v1.DatabaseService/Backup');
  let artifact = '';
  let streamError = null;
  stream.on('data', (chunk) => {
    artifact += encoding.b64decode(chunk.data, 'std', 's');
  });
  stream.on('error', (err) => {
    streamError = err;
  });
  stream.on('end', () => {
    check(artifact, {
      'Backup stream ended without error': () => streamError === null,
      'Backup produced a non-empty artifact': (a) => a.length > 0,
      'Backup artifact starts with the version header': (a) => a.startsWith('{"purser_backup_version":1}\n'),
    });
    client.close();
  });
  stream.write({});
  stream.end();
};
