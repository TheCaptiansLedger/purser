// k6 HTTP/JSON suite for SettingsService. See test/k6/http/job_test.js for
// the pattern this follows — like Job, Settings has no Create/Delete/List-
// with-filter shape (GetSettings/UpdateSettings/ResetSetting instead). See
// docs/adr/0028-layered-settings.md.
import http from 'k6/http';
import { check } from 'k6';
import { options } from '../lib/options.js';
export { options };

const BASE_URL = __ENV.PURSER_HTTP_URL || 'http://localhost:7474';
const SERVICE = `${BASE_URL}/purser.settings.v1.SettingsService`;
const HEADERS = { headers: { 'Content-Type': 'application/json' } };

function invoke(url, body, headers) {
  const res = http.post(url, body, headers);
  console.log(JSON.stringify({ method: url, request: JSON.parse(body), response: res.json() }, null, 2));
  return res;
}

const KEY = 'pipeline.confidence_threshold';

export default () => {
  // GetSettings returns the full key set, including bootstrap-locked keys
  // (never DB-editable) and secret keys (never plaintext).
  let res = invoke(`${SERVICE}/GetSettings`, JSON.stringify({}), HEADERS);
  check(res, {
    'GetSettings status is 200': (r) => r.status === 200,
    'GetSettings returns a non-empty key set': (r) => (r.json('settings') || []).length > 0,
  });
  const settings = res.json('settings');

  const confidenceThreshold = settings.find((s) => s.key === KEY);
  check(confidenceThreshold, {
    'pipeline.confidence_threshold is present and unlocked': (s) => !!s && s.locked !== true,
  });

  const databaseDriver = settings.find((s) => s.key === 'database.driver');
  check(databaseDriver, {
    'database.driver is bootstrap-locked': (s) => !!s && s.locked === true && s.lockReason === 'SETTING_LOCK_REASON_BOOTSTRAP',
  });

  const stashDBAPIKey = settings.find((s) => s.key === 'sources.stashdb.api_key');
  check(stashDBAPIKey, {
    'sources.stashdb.api_key is marked secret': (s) => !!s && s.secret === true,
    // An unset secret's value is the empty string (docs/adr/0028-layered-settings.md),
    // which protojson omits from the response entirely rather than emitting
    // `"value": ""` — so the unset case shows up here as undefined, not ''.
    'sources.stashdb.api_key value is never plaintext': (s) => !s.value || s.value === '********',
  });

  // UpdateSettings writes a new value for an unlocked key and reflects the
  // refreshed source/value immediately (no restart) — see the "Runtime
  // effect without restart" section of docs/adr/0028-layered-settings.md.
  res = invoke(`${SERVICE}/UpdateSettings`, JSON.stringify({ values: { [KEY]: '0.42' }, updateMask: [KEY] }), HEADERS);
  check(res, {
    'UpdateSettings status is 200': (r) => r.status === 200,
    'UpdateSettings returns the new value': (r) => r.json('settings')[0].value === '0.42',
    'UpdateSettings returns source=DB': (r) => r.json('settings')[0].source === 'SETTING_SOURCE_DB',
  });

  res = invoke(`${SERVICE}/GetSettings`, JSON.stringify({}), HEADERS);
  check(res, {
    'GetSettings after UpdateSettings reflects the write': (r) => r.json('settings').find((s) => s.key === KEY).value === '0.42',
  });

  // UpdateSettings on a locked key is rejected wholesale, before any write.
  res = invoke(
    `${SERVICE}/UpdateSettings`,
    JSON.stringify({ values: { 'database.driver': '"postgres"' }, updateMask: ['database.driver'] }),
    HEADERS
  );
  check(res, { 'UpdateSettings on a bootstrap-locked key is 400 (FailedPrecondition)': (r) => r.status === 400 });

  // UpdateSettings on an unknown key is rejected.
  res = invoke(`${SERVICE}/UpdateSettings`, JSON.stringify({ values: { 'no.such.key': '"x"' }, updateMask: ['no.such.key'] }), HEADERS);
  check(res, { 'UpdateSettings on an unknown key is 404 (NotFound)': (r) => r.status === 404 });

  // UpdateSettings with a mask/values mismatch is a validation error.
  res = invoke(
    `${SERVICE}/UpdateSettings`,
    JSON.stringify({ values: { [KEY]: '0.5', 'sources.stashdb.enabled': 'true' }, updateMask: [KEY] }),
    HEADERS
  );
  check(res, { 'UpdateSettings with a mask/values mismatch is 400 (InvalidArgument)': (r) => r.status === 400 });

  // ResetSetting clears the DB override, falling back to the default.
  res = invoke(`${SERVICE}/ResetSetting`, JSON.stringify({ key: KEY }), HEADERS);
  check(res, {
    'ResetSetting status is 200': (r) => r.status === 200,
    'ResetSetting falls back to the default value': (r) => r.json('setting').value === '0.75',
    'ResetSetting reports source=DEFAULT': (r) => r.json('setting').source === 'SETTING_SOURCE_DEFAULT',
  });

  // ResetSetting on a locked key is rejected.
  res = invoke(`${SERVICE}/ResetSetting`, JSON.stringify({ key: 'database.driver' }), HEADERS);
  check(res, { 'ResetSetting on a bootstrap-locked key is 400 (FailedPrecondition)': (r) => r.status === 400 });

  // ResetSetting on an unknown key is rejected.
  res = invoke(`${SERVICE}/ResetSetting`, JSON.stringify({ key: 'no.such.key' }), HEADERS);
  check(res, { 'ResetSetting on an unknown key is 404 (NotFound)': (r) => r.status === 404 });
};
