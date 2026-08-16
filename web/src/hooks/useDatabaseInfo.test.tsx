import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import type { ReactNode } from 'react'
import { DatabaseService } from '../gen/purser/database/v1/database_pb'
import { useDatabaseInfo } from './useDatabaseInfo'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend — same createRouterTransport pattern as useSettings.test.tsx.
function wrapper(mockTransport: ReturnType<typeof createRouterTransport>) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <TransportProvider transport={mockTransport}>
        <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
      </TransportProvider>
    )
  }
}

describe('useDatabaseInfo', () => {
  it('returns database info from a successful GetDatabaseInfo call', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(DatabaseService, {
        getDatabaseInfo: () => ({
          driver: 'badger',
          version: '4.2.0',
          storageSizeBytes: 1_048_576n,
          collectionCounts: { person: 12n },
        }),
      })
    })

    const { result } = renderHook(() => useDatabaseInfo(), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data?.driver).toBe('badger')
    expect(result.current.data?.collectionCounts.person).toBe(12n)
  })

  it('surfaces a ConnectError when GetDatabaseInfo fails', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(DatabaseService, {
        getDatabaseInfo: () => {
          throw new Error('unavailable')
        },
      })
    })

    const { result } = renderHook(() => useDatabaseInfo(), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current.isError).toBe(true))
    expect(result.current.error).toBeTruthy()
  })
})
