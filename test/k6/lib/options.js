// Shared k6 options — a failed check() must fail the process (non-zero
// exit), not just print a red mark in a summary nobody re-reads. See
// docs/adr/0022-k6-ci-enforcement.md. Every script under
// test/k6/{grpc,http,flow}/*.js re-exports this via:
//   import { options } from '../lib/options.js';
//   export { options };
export const options = {
  thresholds: {
    checks: ['rate==1.0'],
  },
};
