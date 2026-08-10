import { describe, expect, it } from 'vitest'
import { create } from '@bufbuild/protobuf'
import { SettingLockReason, SettingSchema, SettingSource } from '../../gen/purser/settings/v1/settings_pb'
import { settingFromProto } from './fromProto'

describe('settingFromProto', () => {
  it('converts an unlocked, DB-sourced setting', () => {
    const proto = create(SettingSchema, {
      key: 'media.path',
      value: '"/data/media"',
      source: SettingSource.DB,
      locked: false,
      lockReason: SettingLockReason.UNSPECIFIED,
      secret: false,
    })

    expect(settingFromProto(proto)).toEqual({
      key: 'media.path',
      value: '"/data/media"',
      source: 'db',
      locked: false,
      lockReason: 'none',
      secret: false,
    })
  })

  it('converts a bootstrap-locked setting', () => {
    const proto = create(SettingSchema, {
      key: 'database.driver',
      value: '"badger"',
      source: SettingSource.DEFAULT,
      locked: true,
      lockReason: SettingLockReason.BOOTSTRAP,
      secret: false,
    })

    expect(settingFromProto(proto)).toEqual({
      key: 'database.driver',
      value: '"badger"',
      source: 'default',
      locked: true,
      lockReason: 'bootstrap',
      secret: false,
    })
  })
})
