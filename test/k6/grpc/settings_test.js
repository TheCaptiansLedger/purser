// k6 gRPC suite for SettingsService. See test/k6/grpc/job_test.js for the
// pattern this follows — like Job, Settings has no Create/Delete/List-
// with-filter shape (GetSettings/UpdateSettings/ResetSetting instead). See
// docs/adr/0028-layered-settings.md.
import grpc from 'k6/net/grpc';
import { check } from 'k6';
import { options } from '../lib/options.js';
export { options };

const ADDR = __ENV.PURSER_GRPC_ADDR || 'localhost:7474';

const client = new grpc.Client();
client.load(['../../../proto'], 'purser/settings/v1/settings.proto');

function invoke(method, request) {
  const res = client.invoke(method, request);
  console.log(JSON.stringify({ method: method, request: request, response: res.message }, null, 2));
  return res;
}

const KEY = 'pipeline.confidence_threshold';

export default () => {
  client.connect(ADDR, { plaintext: true });

  // GetSettings returns the full key set, including bootstrap-locked keys
  // (never DB-editable) and secret keys (never plaintext).
  let res = invoke('purser.settings.v1.SettingsService/GetSettings', {});
  check(res, {
    'GetSettings status is OK': (r) => r && r.status === grpc.StatusOK,
    'GetSettings returns a non-empty key set': (r) => r && r.message && r.message.settings && r.message.settings.length > 0,
  });
  const settings = res.message.settings;

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
    // which the wire encoding omits entirely rather than sending an empty
    // string field — so the unset case shows up here as undefined, not ''.
    'sources.stashdb.api_key value is never plaintext': (s) => !s.value || s.value === '********',
  });

  // UpdateSettings writes a new value for an unlocked key and reflects the
  // refreshed source/value immediately (no restart) — see the "Runtime
  // effect without restart" section of docs/adr/0028-layered-settings.md.
  res = invoke('purser.settings.v1.SettingsService/UpdateSettings', {
    values: { [KEY]: '0.42' },
    updateMask: [KEY],
  });
  check(res, {
    'UpdateSettings status is OK': (r) => r && r.status === grpc.StatusOK,
    'UpdateSettings returns the new value': (r) => r && r.message && r.message.settings && r.message.settings[0].value === '0.42',
    'UpdateSettings returns source=DB': (r) => r.message.settings[0].source === 'SETTING_SOURCE_DB',
  });

  res = invoke('purser.settings.v1.SettingsService/GetSettings', {});
  check(res, {
    'GetSettings after UpdateSettings reflects the write': (r) => r.message.settings.find((s) => s.key === KEY).value === '0.42',
  });

  // UpdateSettings on a locked key is rejected wholesale, before any write.
  res = invoke('purser.settings.v1.SettingsService/UpdateSettings', {
    values: { 'database.driver': '"postgres"' },
    updateMask: ['database.driver'],
  });
  check(res, { 'UpdateSettings on a bootstrap-locked key is FailedPrecondition': (r) => r && r.status === grpc.StatusFailedPrecondition });

  // UpdateSettings on an unknown key is rejected.
  res = invoke('purser.settings.v1.SettingsService/UpdateSettings', {
    values: { 'no.such.key': '"x"' },
    updateMask: ['no.such.key'],
  });
  check(res, { 'UpdateSettings on an unknown key is NotFound': (r) => r && r.status === grpc.StatusNotFound });

  // UpdateSettings with a mask/values mismatch is a validation error.
  res = invoke('purser.settings.v1.SettingsService/UpdateSettings', {
    values: { [KEY]: '0.5', 'sources.stashdb.enabled': 'true' },
    updateMask: [KEY],
  });
  check(res, { 'UpdateSettings with a mask/values mismatch is InvalidArgument': (r) => r && r.status === grpc.StatusInvalidArgument });

  // ResetSetting clears the DB override, falling back to the default.
  res = invoke('purser.settings.v1.SettingsService/ResetSetting', { key: KEY });
  check(res, {
    'ResetSetting status is OK': (r) => r && r.status === grpc.StatusOK,
    'ResetSetting falls back to the default value': (r) => r && r.message && r.message.setting.value === '0.75',
    'ResetSetting reports source=DEFAULT': (r) => r.message.setting.source === 'SETTING_SOURCE_DEFAULT',
  });

  // ResetSetting on a locked key is rejected.
  res = invoke('purser.settings.v1.SettingsService/ResetSetting', { key: 'database.driver' });
  check(res, { 'ResetSetting on a bootstrap-locked key is FailedPrecondition': (r) => r && r.status === grpc.StatusFailedPrecondition });

  // ResetSetting on an unknown key is rejected.
  res = invoke('purser.settings.v1.SettingsService/ResetSetting', { key: 'no.such.key' });
  check(res, { 'ResetSetting on an unknown key is NotFound': (r) => r && r.status === grpc.StatusNotFound });
};
