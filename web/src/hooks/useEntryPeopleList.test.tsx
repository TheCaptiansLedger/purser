import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import type { ReactNode } from 'react'
import { EntryPersonService } from '../gen/purser/domain/v1/entry_person_pb'
import { useEntryPeopleList } from './useEntryPeopleList'

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

describe('useEntryPeopleList', () => {
  it('lists a person across every LibraryEntry they are credited on', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(EntryPersonService, {
        listEntryPeople: req => {
          expect(req.libraryEntryId).toBe('')
          expect(req.personId).toBe('person-1')
          return { entryPeople: [{ libraryEntryId: 'entry-1', personId: 'person-1', role: 'Director' }] }
        },
      })
    })

    const { result } = renderHook(() => useEntryPeopleList('person-1'), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data?.entryPeople).toHaveLength(1)
  })

  it('skips the call when personId is empty', () => {
    const mockTransport = createRouterTransport(() => {})

    const { result } = renderHook(() => useEntryPeopleList(''), { wrapper: wrapper(mockTransport) })

    expect(result.current.fetchStatus).toBe('idle')
  })
})
