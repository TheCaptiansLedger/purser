import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import type { ReactNode } from 'react'
import { ItemPersonService } from '../gen/purser/domain/v1/item_person_pb'
import { useItemPeopleList } from './useItemPeopleList'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend.
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

describe('useItemPeopleList', () => {
  it('lists a person across every Item they are credited on', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(ItemPersonService, {
        listItemPeople: req => {
          expect(req.itemId).toBe('')
          expect(req.personId).toBe('person-1')
          return { itemPeople: [{ itemId: 'item-1', personId: 'person-1', role: 'Guest Vocalist' }] }
        },
      })
    })

    const { result } = renderHook(() => useItemPeopleList('person-1'), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data?.itemPeople).toHaveLength(1)
  })

  it('skips the call when personId is empty', () => {
    const mockTransport = createRouterTransport(() => {})

    const { result } = renderHook(() => useItemPeopleList(''), { wrapper: wrapper(mockTransport) })

    expect(result.current.fetchStatus).toBe('idle')
  })
})
