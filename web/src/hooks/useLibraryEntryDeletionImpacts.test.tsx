import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { describe, expect, it } from 'vitest'
import { LibraryEntryService } from '../gen/purser/domain/v1/library_entry_pb'
import { useLibraryEntryDeletionImpacts } from './useLibraryEntryDeletionImpacts'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend — same createRouterTransport pattern as
// useLibraryEntryImages.test.tsx, the fan-out hook this one mirrors.
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

describe('useLibraryEntryDeletionImpacts', () => {
  it('sums each artist own GetLibraryEntryDeletionImpact by kind, keeping the Blocking flag', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(LibraryEntryService, {
        getLibraryEntryDeletionImpact: request => {
          if (request.id === 'a1') {
            return { impacts: [{ kind: 'group', label: 'Groups', count: 2, blocking: true }] }
          }
          return { impacts: [{ kind: 'group', label: 'Groups', count: 3, blocking: true }] }
        },
      })
    })

    const { result } = renderHook(() => useLibraryEntryDeletionImpacts(['a1', 'a2']), {
      wrapper: wrapper(mockTransport),
    })

    await waitFor(() => expect(result.current.isPending).toBe(false))
    expect(result.current.rows).toEqual([{ kind: 'group', label: 'Groups', count: 5, blocking: true }])
  })

  it('returns no rows and stays not-pending for an empty selection', () => {
    const mockTransport = createRouterTransport(() => {})

    const { result } = renderHook(() => useLibraryEntryDeletionImpacts([]), { wrapper: wrapper(mockTransport) })

    expect(result.current).toEqual({ isPending: false, rows: [] })
  })
})
