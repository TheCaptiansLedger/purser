import { SettingLockReason as ProtoLockReason, SettingSource as ProtoSource } from '../../gen/purser/settings/v1/settings_pb'
import type { Setting as ProtoSetting } from '../../gen/purser/settings/v1/settings_pb'
import type { Setting, SettingLockReason, SettingSource } from '../../types'

const SOURCE_BY_PROTO: Record<ProtoSource, SettingSource> = {
  [ProtoSource.UNSPECIFIED]: 'unspecified',
  [ProtoSource.DEFAULT]: 'default',
  [ProtoSource.ENV]: 'env',
  [ProtoSource.YAML]: 'yaml',
  [ProtoSource.DB]: 'db',
}

const LOCK_REASON_BY_PROTO: Record<ProtoLockReason, SettingLockReason> = {
  [ProtoLockReason.UNSPECIFIED]: 'none',
  [ProtoLockReason.BOOTSTRAP]: 'bootstrap',
  [ProtoLockReason.OPERATOR]: 'operator',
}

// settingFromProto converts one wire purser.settings.v1.Setting into the
// plain Setting app code is written against (web/src/types/index.ts) —
// enum numbers become the string literals ADR 0004's hand-maintained
// types layer is meant to shield components from.
export function settingFromProto(setting: ProtoSetting): Setting {
  return {
    key: setting.key,
    value: setting.value,
    source: SOURCE_BY_PROTO[setting.source],
    locked: setting.locked,
    lockReason: LOCK_REASON_BY_PROTO[setting.lockReason],
    secret: setting.secret,
  }
}
