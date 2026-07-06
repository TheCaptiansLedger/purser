import http from 'k6/http';

export const BASE_URL = __ENV.BASE_URL || 'http://localhost:7474/api/v1';

// Tell k6 that 2xx, 404, and 204 are all expected responses.
// Without this, deliberate 404 assertions count as "failed" requests.
http.setResponseCallback(http.expectedStatuses(200, 201, 204, 400, 404));

// Default options: functional test (1 VU, 1 iteration).
// Override for load testing:
//   k6 run --vus 50 --duration 30s <script>
export const options = {
  vus: 1,
  iterations: 1,
  thresholds: {
    http_req_failed: ['rate<0.01'],
    http_req_duration: ['p(95)<2000'],
  },
};

export function check200(res, label) {
  if (res.status !== 200) {
    console.error(`FAIL ${label}: expected 200, got ${res.status} — ${res.body}`);
    return false;
  }
  return true;
}

export function check201(res, label) {
  if (res.status !== 201) {
    console.error(`FAIL ${label}: expected 201, got ${res.status} — ${res.body}`);
    return false;
  }
  return true;
}

export function check204(res, label) {
  if (res.status !== 204) {
    console.error(`FAIL ${label}: expected 204, got ${res.status} — ${res.body}`);
    return false;
  }
  return true;
}

export function check404(res, label) {
  if (res.status !== 404) {
    console.error(`FAIL ${label}: expected 404, got ${res.status} — ${res.body}`);
    return false;
  }
  return true;
}

export const JSON_HEADERS = { 'Content-Type': 'application/json' };
