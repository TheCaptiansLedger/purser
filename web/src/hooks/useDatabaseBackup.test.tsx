import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { act, renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { DatabaseService } from '../gen/purser/database/v1/database_pb'
import { useDatabaseBackup } from './useDatabaseBackup'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend — same createRouterTransport pattern as useWatchJob.test.tsx.
function wrapper(mockTransport: ReturnType<typeof createRouterTransport>) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <TransportProvider transport={mockTransport}>{children}</TransportProvider>
  }
}

// jsdom doesn't implement the Blob URL / anchor-download APIs
// downloadBlob relies on — stub the pieces it touches so the hook is
// testable without a real browser.
beforeEach(() => {
  URL.createObjectURL = vi.fn(() => 'blob:mock-url')
  URL.revokeObjectURL = vi.fn()
})

afterEach(() => {
  vi.restoreAllMocks()
})

describe('useDatabaseBackup', () => {
  it('accumulates streamed chunks and triggers a download on completion', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(DatabaseService, {
        backup: async function* () {
          yield { data: new Uint8Array([1, 2, 3]) }
          yield { data: new Uint8Array([4, 5]) }
        },
      })
    })

    const clickSpy = vi.fn()
    const originalCreateElement = document.createElement.bind(document)
    vi.spyOn(document, 'createElement').mockImplementation(tag => {
      const el = originalCreateElement(tag)
      if (tag === 'a') el.click = clickSpy
      return el
    })

    const { result } = renderHook(() => useDatabaseBackup(), { wrapper: wrapper(mockTransport) })

    await act(() => result.current.runBackup())

    await waitFor(() => expect(result.current.phase).toBe('done'))
    expect(result.current.bytesReceived).toBe(5)
    expect(result.current.error).toBeUndefined()
    expect(clickSpy).toHaveBeenCalledOnce()
    expect(URL.createObjectURL).toHaveBeenCalledOnce()
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:mock-url')
  })

  it('surfaces a ConnectError when the stream fails', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(DatabaseService, {
        // eslint-disable-next-line require-yield
        backup: async function* () {
          throw new Error('unavailable')
        },
      })
    })

    const { result } = renderHook(() => useDatabaseBackup(), { wrapper: wrapper(mockTransport) })

    await act(() => result.current.runBackup())

    await waitFor(() => expect(result.current.phase).toBe('error'))
    expect(result.current.error).toBeTruthy()
  })
})
