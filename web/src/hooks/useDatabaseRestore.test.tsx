import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { act, renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { DatabaseService } from '../gen/purser/database/v1/database_pb'
import { useDatabaseRestore } from './useDatabaseRestore'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend — same createRouterTransport pattern as useWatchJob.test.tsx.
function wrapper(mockTransport: ReturnType<typeof createRouterTransport>) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <TransportProvider transport={mockTransport}>{children}</TransportProvider>
  }
}

afterEach(() => {
  vi.useRealTimers()
})

describe('useDatabaseRestore', () => {
  it('uploads a file in chunks, then reports ready once the server answers again', async () => {
    let chunksReceived = 0
    let bytesReceived = 0
    const mockTransport = createRouterTransport(router => {
      router.service(DatabaseService, {
        restore: async stream => {
          for await (const chunk of stream) {
            chunksReceived++
            bytesReceived += chunk.data.length
          }
          return { restartRequired: true }
        },
        getDatabaseInfo: () => ({
          driver: 'badger',
          version: '4.2.0',
          storageSizeBytes: 0n,
          collectionCounts: {},
        }),
      })
    })

    // Bigger than the hook's 64KB chunk size, so the upload is split into
    // more than one RestoreRequest.
    const bytes = new Uint8Array(64 * 1024 + 10).fill(7)
    const file = new File([bytes], 'backup.jsonl')

    const { result } = renderHook(() => useDatabaseRestore(), { wrapper: wrapper(mockTransport) })

    await act(() => result.current.runRestore(file))

    await waitFor(() => expect(result.current.phase).toBe('ready'))
    expect(chunksReceived).toBeGreaterThan(1)
    expect(bytesReceived).toBe(bytes.length)
    expect(result.current.bytesSent).toBe(bytes.length)
    expect(result.current.totalBytes).toBe(bytes.length)
    expect(result.current.error).toBeUndefined()
  })

  it('surfaces a ConnectError when the upload itself fails', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(DatabaseService, {
        restore: async () => {
          throw new Error('invalid backup stream')
        },
      })
    })

    const file = new File([new Uint8Array([1, 2, 3])], 'backup.jsonl')
    const { result } = renderHook(() => useDatabaseRestore(), { wrapper: wrapper(mockTransport) })

    await act(() => result.current.runRestore(file))

    await waitFor(() => expect(result.current.phase).toBe('error'))
    expect(result.current.error).toBeTruthy()
  })

  it('retries GetDatabaseInfo until the restarted server answers', async () => {
    vi.useFakeTimers()
    let infoAttempts = 0
    const mockTransport = createRouterTransport(router => {
      router.service(DatabaseService, {
        restore: async stream => {
          for await (const _chunk of stream) {
            /* drain */
          }
          return { restartRequired: true }
        },
        getDatabaseInfo: () => {
          infoAttempts++
          if (infoAttempts < 3) {
            throw new Error('connection refused')
          }
          return { driver: 'badger', version: '4.2.0', storageSizeBytes: 0n, collectionCounts: {} }
        },
      })
    })

    const file = new File([new Uint8Array([1, 2, 3])], 'backup.jsonl')
    const { result } = renderHook(() => useDatabaseRestore(), { wrapper: wrapper(mockTransport) })

    await act(async () => {
      const runPromise = result.current.runRestore(file)
      await vi.advanceTimersByTimeAsync(2100)
      await runPromise
    })

    expect(result.current.phase).toBe('ready')
    expect(infoAttempts).toBe(3)
  })
})
