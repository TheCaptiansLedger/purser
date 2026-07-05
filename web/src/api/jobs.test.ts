import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { pollForItemId } from './jobs'

const ORIGIN = 'http://localhost'

beforeEach(() => {
  vi.stubGlobal('window', { location: { origin: ORIGIN } })
  vi.useFakeTimers()
})

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

function jobResponse(status: string, itemId?: string) {
  return {
    ok: true,
    status: 200,
    json: () =>
      Promise.resolve({
        id: 'job-1',
        name: 'ImportItem',
        status,
        current: 0,
        total: 0,
        createdAt: new Date().toISOString(),
        startedAt: null,
        completedAt: null,
        ...(itemId ? { result: { item_id: itemId } } : {}),
      }),
  }
}

describe('pollForItemId', () => {
  it('returns item_id when job completes on first poll', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jobResponse('completed', 'item-abc')))

    const promise = pollForItemId('job-1')
    await vi.advanceTimersByTimeAsync(500)

    await expect(promise).resolves.toBe('item-abc')
  })

  it('retries when job is still running and succeeds on second poll', async () => {
    const fetch = vi.fn()
      .mockResolvedValueOnce(jobResponse('running'))
      .mockResolvedValueOnce(jobResponse('completed', 'item-xyz'))
    vi.stubGlobal('fetch', fetch)

    const promise = pollForItemId('job-1')
    await vi.advanceTimersByTimeAsync(500)   // first wait
    await vi.advanceTimersByTimeAsync(1000)  // second wait (doubled delay)

    await expect(promise).resolves.toBe('item-xyz')
    expect(fetch).toHaveBeenCalledTimes(2)
  })

  it('throws when job fails', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ id: 'job-1', name: 'ImportItem', status: 'failed', error: 'no studio', current: 0, total: 0, createdAt: '' }),
    }))

    const settled = pollForItemId('job-1').then(v => ({ ok: true as const, value: v }), e => ({ ok: false as const, error: e as Error }))
    await vi.advanceTimersByTimeAsync(500)
    const result = await settled
    expect(result.ok).toBe(false)
    expect((result as { ok: false; error: Error }).error.message).toContain('no studio')
  })

  it('throws when completed job has no item_id', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jobResponse('completed')))

    const settled = pollForItemId('job-1').then(v => ({ ok: true as const, value: v }), e => ({ ok: false as const, error: e as Error }))
    await vi.advanceTimersByTimeAsync(500)
    const result = await settled
    expect(result.ok).toBe(false)
    expect((result as { ok: false; error: Error }).error.message).toContain('no item_id')
  })

  it('throws after max retries if job never completes', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jobResponse('running')))

    const settled = pollForItemId('job-1').then(v => ({ ok: true as const, value: v }), e => ({ ok: false as const, error: e as Error }))
    // Advance past all 8 retries (500 + 1000 + 2000 + 4000 + 8000 + 10000 + 10000 + 10000 = 45500ms)
    await vi.advanceTimersByTimeAsync(60_000)
    const result = await settled
    expect(result.ok).toBe(false)
    expect((result as { ok: false; error: Error }).error.message).toContain('did not complete')
  })
})
