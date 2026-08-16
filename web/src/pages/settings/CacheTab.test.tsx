import { fireEvent, render, screen } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { useCacheStats } from '../../hooks/useCacheStats'
import { useFlushCacheMutation } from '../../hooks/useFlushCacheMutation'
import { ListCacheStatsResponseSchema } from '../../gen/purser/cache/v1/cache_pb'
import { CacheTab } from './CacheTab'

// Page-level test: composition only, per ADR 0004 — cacheStatsFromProto's
// own conversion is tested in cacheFromProto.test.ts, hitRate in
// cacheFormat.test.ts, and each hook's own RPC behavior in
// useCacheStats.test.tsx/useFlushCacheMutation.test.tsx. Both hooks are
// mocked directly so every state is reachable deterministically.
vi.mock('../../hooks/useCacheStats')
const mockUseCacheStats = vi.mocked(useCacheStats)

vi.mock('../../hooks/useFlushCacheMutation')
const mockUseFlushCacheMutation = vi.mocked(useFlushCacheMutation)

const refetch = vi.fn()

function mockStats(caches: { name: string; items: number; bytes: bigint; hits: bigint; misses: bigint }[]) {
  mockUseCacheStats.mockReturnValue({
    isPending: false,
    isError: false,
    data: create(ListCacheStatsResponseSchema, { caches }),
    error: null,
    refetch,
  } as unknown as ReturnType<typeof useCacheStats>)
}

const mutate = vi.fn()

// Overrides are loosely typed and the merged result cast wholesale, same
// as DatabaseTab.test.tsx's mockActions casts.
function mockFlush(overrides: Record<string, unknown> = {}) {
  mockUseFlushCacheMutation.mockReturnValue({
    isPending: false,
    isError: false,
    error: undefined,
    variables: undefined,
    mutate,
    ...overrides,
  } as unknown as ReturnType<typeof useFlushCacheMutation>)
}

afterEach(() => {
  vi.clearAllMocks()
})

describe('CacheTab', () => {
  it('renders nothing while cache stats are pending', () => {
    mockUseCacheStats.mockReturnValue({
      isPending: true,
      isError: false,
      data: undefined,
      error: null,
      refetch,
    } as unknown as ReturnType<typeof useCacheStats>)
    mockFlush()

    const { container } = render(<CacheTab />)
    expect(container.textContent).toBe('')
  })

  it('shows an actionable error when ListCacheStats fails', () => {
    mockUseCacheStats.mockReturnValue({
      isPending: false,
      isError: true,
      data: undefined,
      error: { message: 'unavailable' },
      refetch,
    } as unknown as ReturnType<typeof useCacheStats>)
    mockFlush()

    render(<CacheTab />)
    expect(screen.getByRole('alert')).toHaveTextContent("Couldn't load cache stats (unavailable)")
  })

  it('shows an empty state when no caches are registered', () => {
    mockStats([])
    mockFlush()

    render(<CacheTab />)
    expect(screen.getByText('No caches registered.')).toBeInTheDocument()
  })

  it('renders one card per cache with hit rate, hits/misses, entries, and memory', () => {
    mockStats([{ name: 'musicbrainz', items: 12, bytes: 4096n, hits: 80n, misses: 20n }])
    mockFlush()

    render(<CacheTab />)
    expect(screen.getByText('musicbrainz')).toBeInTheDocument()
    expect(screen.getByText('80% hit rate')).toBeInTheDocument()
    expect(screen.getByText('80')).toBeInTheDocument()
    expect(screen.getByText('20')).toBeInTheDocument()
    expect(screen.getByText('12')).toBeInTheDocument()
    expect(screen.getByText('4.0 KB')).toBeInTheDocument()
  })

  it('renders one card per cache when several caches are registered', () => {
    mockStats([
      { name: 'musicbrainz', items: 12, bytes: 4096n, hits: 80n, misses: 20n },
      { name: 'stashdb', items: 0, bytes: 0n, hits: 0n, misses: 0n },
    ])
    mockFlush()

    render(<CacheTab />)
    expect(screen.getByText('musicbrainz')).toBeInTheDocument()
    expect(screen.getByText('stashdb')).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: /Flush/ })).toHaveLength(2)
  })

  it('calls mutate with the cache name when Flush is clicked, then refetches on success', () => {
    mockStats([{ name: 'musicbrainz', items: 12, bytes: 4096n, hits: 80n, misses: 20n }])
    mockFlush()
    mutate.mockImplementation((_vars, opts?: { onSuccess?: () => void }) => opts?.onSuccess?.())

    render(<CacheTab />)
    fireEvent.click(screen.getByRole('button', { name: /Flush/ }))

    expect(mutate).toHaveBeenCalledWith({ name: 'musicbrainz' }, expect.objectContaining({ onSuccess: expect.any(Function) }))
    expect(refetch).toHaveBeenCalledOnce()
  })

  it('disables the flush button for a cache with no entries', () => {
    mockStats([{ name: 'stashdb', items: 0, bytes: 0n, hits: 0n, misses: 0n }])
    mockFlush()

    render(<CacheTab />)
    expect(screen.getByRole('button', { name: /Flush/ })).toBeDisabled()
  })

  it('shows a flush error', () => {
    mockStats([{ name: 'musicbrainz', items: 12, bytes: 4096n, hits: 80n, misses: 20n }])
    mockFlush({ isError: true, error: { message: 'cache not found' } })

    render(<CacheTab />)
    expect(screen.getByText('Flush failed: cache not found')).toBeInTheDocument()
  })
})
