import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { getDeletionPreview, delEntityConfirmed, delConfirmed, del } from './client'

// req() uses window.location.origin to resolve relative paths.
// Stub it so tests run in the node environment without a real DOM.
const ORIGIN = 'http://localhost'

function mockFetch(status: number, body: unknown) {
  const res = {
    ok: status >= 200 && status < 300,
    status,
    statusText: status === 200 ? 'OK' : 'Error',
    json: () => Promise.resolve(body),
  }
  return vi.fn().mockResolvedValue(res)
}

beforeEach(() => {
  vi.stubGlobal('window', { location: { origin: ORIGIN } })
  vi.stubGlobal('fetch', mockFetch(200, {}))
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('getDeletionPreview', () => {
  it('calls /api/v1/{resource}/{id}/deletion-preview', async () => {
    const fetch = mockFetch(200, { mode: 'destroy', summary: 'test', impacts: [] })
    vi.stubGlobal('fetch', fetch)

    await getDeletionPreview('library-entries', 'abc123')

    expect(fetch).toHaveBeenCalledOnce()
    const url: string = fetch.mock.calls[0][0]
    expect(url).toBe(`${ORIGIN}/api/v1/library-entries/abc123/deletion-preview`)
  })

  it('never contains /api/v1 more than once', async () => {
    const fetch = mockFetch(200, { mode: 'unlink', summary: '', impacts: [] })
    vi.stubGlobal('fetch', fetch)

    await getDeletionPreview('people', 'person-1')

    const url: string = fetch.mock.calls[0][0]
    const count = (url.match(/api\/v1/g) ?? []).length
    expect(count).toBe(1)
  })
})

describe('delEntityConfirmed', () => {
  it('calls DELETE /api/v1/{resource}/{id} with Purser-Confirm-Delete header', async () => {
    const fetch = mockFetch(204, null)
    vi.stubGlobal('fetch', fetch)

    await delEntityConfirmed('groups', 'grp-42')

    expect(fetch).toHaveBeenCalledOnce()
    const [url, init] = fetch.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/groups/grp-42')
    expect(init.method).toBe('DELETE')
    expect((init.headers as Record<string, string>)['Purser-Confirm-Delete']).toBe('yes')
  })

  it('never contains /api/v1 more than once in the URL', async () => {
    const fetch = mockFetch(204, null)
    vi.stubGlobal('fetch', fetch)

    await delEntityConfirmed('items', 'item-99')

    const url: string = fetch.mock.calls[0][0]
    const count = (url.match(/api\/v1/g) ?? []).length
    expect(count).toBe(1)
  })
})

describe('delConfirmed', () => {
  it('sends DELETE with Purser-Confirm-Delete header', async () => {
    const fetch = mockFetch(204, null)
    vi.stubGlobal('fetch', fetch)

    await delConfirmed('/items/item-1')

    const [, init] = fetch.mock.calls[0] as [string, RequestInit]
    expect(init.method).toBe('DELETE')
    expect((init.headers as Record<string, string>)['Purser-Confirm-Delete']).toBe('yes')
  })
})

describe('del', () => {
  it('sends DELETE without Purser-Confirm-Delete header', async () => {
    const fetch = mockFetch(204, null)
    vi.stubGlobal('fetch', fetch)

    await del('/tags/tag-1')

    const [, init] = fetch.mock.calls[0] as [string, RequestInit]
    expect(init.method).toBe('DELETE')
    expect(init.headers).toBeUndefined()
  })
})
