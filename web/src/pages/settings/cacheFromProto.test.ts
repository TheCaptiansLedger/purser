import { describe, expect, it } from 'vitest'
import { create } from '@bufbuild/protobuf'
import { ListCacheStatsResponseSchema } from '../../gen/purser/cache/v1/cache_pb'
import { cacheStatsFromProto } from './cacheFromProto'

describe('cacheStatsFromProto', () => {
  it('converts each cache, bigint fields to number, preserving order', () => {
    const proto = create(ListCacheStatsResponseSchema, {
      caches: [
        { name: 'musicbrainz', items: 12, bytes: 4096n, hits: 80n, misses: 20n },
        { name: 'stashdb', items: 0, bytes: 0n, hits: 0n, misses: 0n },
      ],
    })

    expect(cacheStatsFromProto(proto)).toEqual([
      { name: 'musicbrainz', items: 12, bytes: 4096, hits: 80, misses: 20 },
      { name: 'stashdb', items: 0, bytes: 0, hits: 0, misses: 0 },
    ])
  })

  it('converts an empty caches list to an empty array', () => {
    const proto = create(ListCacheStatsResponseSchema, { caches: [] })
    expect(cacheStatsFromProto(proto)).toEqual([])
  })
})
