import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import type { ReactNode } from 'react'
import { PersonService } from '../gen/purser/domain/v1/person_pb'
import { usePeopleList } from './usePeopleList'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend — same pattern as useJobsList.test.tsx.
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

describe('usePeopleList', () => {
  it('fetches the first page and reports whether more pages exist', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(PersonService, {
        listPeople: () => ({ people: [{ id: 'p1', name: 'Jane Doe' }], nextPageToken: 'p1' }),
      })
    })

    const { result } = renderHook(() => usePeopleList(''), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data?.pages[0].people).toHaveLength(1)
    expect(result.current.hasNextPage).toBe(true)
  })

  it('appends the next page, keyed by the previous nextPageToken, on fetchNextPage', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(PersonService, {
        listPeople: request => {
          if (request.pageToken === '') {
            return { people: [{ id: 'p1', name: 'Jane Doe' }], nextPageToken: 'p1' }
          }
          expect(request.pageToken).toBe('p1')
          return { people: [{ id: 'p2', name: 'Stevie Nicks' }], nextPageToken: '' }
        },
      })
    })

    const { result } = renderHook(() => usePeopleList(''), { wrapper: wrapper(mockTransport) })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    const next = await result.current.fetchNextPage()
    expect(next.data?.pages).toHaveLength(2)
    expect(next.data?.pages[1].people[0].id).toBe('p2')
    expect(next.hasNextPage).toBe(false)
  })

  it('sends the name filter through to ListPeopleRequest', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(PersonService, {
        listPeople: request => {
          expect(request.name).toBe('nicks')
          return { people: [{ id: 'p2', name: 'Stevie Nicks' }], nextPageToken: '' }
        },
      })
    })

    const { result } = renderHook(() => usePeopleList('nicks'), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data?.pages[0].people[0].name).toBe('Stevie Nicks')
  })
})
