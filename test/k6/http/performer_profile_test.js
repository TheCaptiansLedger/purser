// k6 HTTP/JSON suite for PerformerProfileService. See
// test/k6/http/person_test.js for the pattern this follows.
import http from 'k6/http';
import { check } from 'k6';

const BASE_URL = __ENV.PURSER_HTTP_URL || 'http://localhost:7474';
const SERVICE = `${BASE_URL}/purser.afterdark.v1.PerformerProfileService`;
const HEADERS = { headers: { 'Content-Type': 'application/json' } };

function invoke(url, body, headers) {
  const res = http.post(url, body, headers);
  console.log(JSON.stringify({ method: url, request: JSON.parse(body), response: res.json() }, null, 2));
  return res;
}

export default () => {
  const personId = `k6-http-performer-${__VU}-${__ITER}-${Date.now()}`;

  let res = invoke(
    `${SERVICE}/CreatePerformerProfile`,
    JSON.stringify({ performerProfile: { personId: personId, cupSize: '34', bandSize: 'C', careerStartYear: 2015 } }),
    HEADERS
  );
  check(res, {
    'CreatePerformerProfile status is 200': (r) => r.status === 200,
    'CreatePerformerProfile returns the person id': (r) => r.json('performerProfile.personId') === personId,
  });

  res = invoke(`${SERVICE}/GetPerformerProfile`, JSON.stringify({ personId: personId }), HEADERS);
  check(res, {
    'GetPerformerProfile status is 200': (r) => r.status === 200,
    'GetPerformerProfile returns the created cup size': (r) => r.json('performerProfile.cupSize') === '34',
  });

  res = invoke(
    `${SERVICE}/UpdatePerformerProfile`,
    JSON.stringify({ performerProfile: { personId: personId, cupSize: '36' }, updateMask: 'cupSize' }),
    HEADERS
  );
  check(res, {
    'UpdatePerformerProfile status is 200': (r) => r.status === 200,
    'UpdatePerformerProfile applied the field-masked cup size': (r) => r.json('performerProfile.cupSize') === '36',
  });

  res = invoke(`${SERVICE}/ListPerformerProfiles`, JSON.stringify({ pageSize: 10 }), HEADERS);
  check(res, {
    'ListPerformerProfiles status is 200': (r) => r.status === 200,
    'ListPerformerProfiles includes the created profile': (r) =>
      (r.json('performerProfiles') || []).some((p) => p.personId === personId),
  });

  res = invoke(`${SERVICE}/DeletePerformerProfile`, JSON.stringify({ personId: personId }), HEADERS);
  check(res, { 'DeletePerformerProfile status is 200': (r) => r.status === 200 });

  res = invoke(`${SERVICE}/GetPerformerProfile`, JSON.stringify({ personId: personId }), HEADERS);
  check(res, { 'GetPerformerProfile after Delete is 404 (NotFound)': (r) => r.status === 404 });
};
