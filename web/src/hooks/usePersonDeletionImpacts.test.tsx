import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { describe, expect, it } from 'vitest'
import { PersonService } from '../gen/purser/domain/v1/person_pb'
import { usePersonDeletionImpacts } from './usePersonDeletionImpacts'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend — same createRouterTransport pattern
// useGroupDeletionImpacts.test.tsx already established for the sibling
// hook.
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

describe('usePersonDeletionImpacts', () => {
  it('sums each person own GetPersonDeletionImpact by kind', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(PersonService, {
        getPersonDeletionImpact: request => {
          if (request.id === 'p1') {
            return { impacts: [{ kind: 'entry_person', label: 'Credits (Library Entries)', count: 2, blocking: false }] }
          }
          return { impacts: [{ kind: 'entry_person', label: 'Credits (Library Entries)', count: 3, blocking: false }] }
        },
      })
    })

    const { result } = renderHook(() => usePersonDeletionImpacts(['p1', 'p2']), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current.isPending).toBe(false))
    expect(result.current.rows).toEqual([
      { kind: 'entry_person', label: 'Credits (Library Entries)', count: 5, blocking: false },
    ])
  })

  it('returns no rows and stays not-pending for an empty selection', () => {
    const mockTransport = createRouterTransport(() => {})

    const { result } = renderHook(() => usePersonDeletionImpacts([]), { wrapper: wrapper(mockTransport) })

    expect(result.current).toEqual({ isPending: false, rows: [] })
  })
})
