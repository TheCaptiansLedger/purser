import { describe, expect, it } from 'vitest'
import { create } from '@bufbuild/protobuf'
import { GetDatabaseInfoResponseSchema } from '../../gen/purser/database/v1/database_pb'
import { databaseInfoFromProto } from './databaseFromProto'

describe('databaseInfoFromProto', () => {
  it('converts driver/version/storage size and per-collection counts, bigint to number', () => {
    const proto = create(GetDatabaseInfoResponseSchema, {
      driver: 'badger',
      version: '4.2.0',
      storageSizeBytes: 1_048_576n,
      collectionCounts: { person: 12n, tag: 340n },
    })

    expect(databaseInfoFromProto(proto)).toEqual({
      driver: 'badger',
      version: '4.2.0',
      storageSizeBytes: 1_048_576,
      collectionCounts: { person: 12, tag: 340 },
    })
  })

  it('converts an empty collectionCounts map to an empty object', () => {
    const proto = create(GetDatabaseInfoResponseSchema, {
      driver: 'sqlite',
      version: 'unknown',
      storageSizeBytes: 0n,
      collectionCounts: {},
    })

    expect(databaseInfoFromProto(proto)).toEqual({
      driver: 'sqlite',
      version: 'unknown',
      storageSizeBytes: 0,
      collectionCounts: {},
    })
  })
})
