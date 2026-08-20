import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { describe, expect, it } from 'vitest'
import { GroupService } from '../gen/purser/domain/v1/group_pb'
import { useGroupDeletionImpacts } from './useGroupDeletionImpacts'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend — same createRouterTransport pattern
// useLibraryEntryDeletionImpacts.test.tsx already established for the
// sibling hook.
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

describe('useGroupDeletionImpacts', () => {
  it('sums each album own GetGroupDeletionImpact by kind, and drops zero-count kinds', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(GroupService, {
        getGroupDeletionImpact: request => {
          if (request.id === 'g1') {
            return {
              impacts: [
                { kind: 'item', label: 'Items (will be detached, not deleted)', count: 4, blocking: false },
                { kind: 'image', label: 'Images', count: 0, blocking: false },
              ],
            }
          }
          return {
            impacts: [
              { kind: 'item', label: 'Items (will be detached, not deleted)', count: 6, blocking: false },
              { kind: 'image', label: 'Images', count: 0, blocking: false },
            ],
          }
        },
      })
    })

    const { result } = renderHook(() => useGroupDeletionImpacts(['g1', 'g2']), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current.isPending).toBe(false))
    expect(result.current.rows).toEqual([
      { kind: 'item', label: 'Items (will be detached, not deleted)', count: 10, blocking: false },
    ])
  })

  it('returns no rows and stays not-pending for an empty selection', () => {
    const mockTransport = createRouterTransport(() => {})

    const { result } = renderHook(() => useGroupDeletionImpacts([]), { wrapper: wrapper(mockTransport) })

    expect(result.current).toEqual({ isPending: false, rows: [] })
  })
})
