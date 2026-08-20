import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { timestampFromDate } from '@bufbuild/protobuf/wkt'
import { describe, expect, it } from 'vitest'
import type { ReactNode } from 'react'
import { EntryPersonService } from '../gen/purser/domain/v1/entry_person_pb'
import { PersonService } from '../gen/purser/domain/v1/person_pb'
import { useArtistMembers } from './useArtistMembers'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend — same pattern as usePeopleList.test.tsx.
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

// A person with a single, still-open role — the first configuration:
// exercises the "current" path and the raw role/label split #724's
// Edit/Remove dialogs key on.
describe('useArtistMembers — single open role', () => {
  it('resolves the row to a current ArtistMember carrying both the raw role and its formatted label', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(EntryPersonService, {
        listEntryPeople: () => ({
          entryPeople: [
            {
              libraryEntryId: 'artist-1',
              personId: 'person-1',
              role: 'vocalist',
              startDate: timestampFromDate(new Date('1975-01-01')),
            },
          ],
        }),
      })
      router.service(PersonService, { getPerson: () => ({ person: { id: 'person-1', name: 'Stevie Nicks' } }) })
    })

    const { result } = renderHook(() => useArtistMembers('artist-1'), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current.isPending).toBe(false))
    expect(result.current.members).toEqual([
      {
        personId: 'person-1',
        name: 'Stevie Nicks',
        imageId: undefined,
        roles: [{ role: 'vocalist', label: 'vocalist (since 1975)', startDate: expect.anything(), endDate: undefined }],
        former: false,
      },
    ])
  })
})

// A person with two roles, one still-open — the second configuration:
// exercises the multi-role grouping and the "any open role keeps them
// current" rule.
describe('useArtistMembers — multiple roles, one still open', () => {
  it('groups both roles under one member and reports them current', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(EntryPersonService, {
        listEntryPeople: () => ({
          entryPeople: [
            {
              libraryEntryId: 'artist-1',
              personId: 'person-1',
              role: 'guitarist',
              startDate: timestampFromDate(new Date('1968-01-01')),
              endDate: timestampFromDate(new Date('1980-01-01')),
            },
            { libraryEntryId: 'artist-1', personId: 'person-1', role: 'producer' },
          ],
        }),
      })
      router.service(PersonService, { getPerson: () => ({ person: { id: 'person-1', name: 'Peter Green' } }) })
    })

    const { result } = renderHook(() => useArtistMembers('artist-1'), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current.isPending).toBe(false))
    expect(result.current.members).toHaveLength(1)
    expect(result.current.members[0].roles.map(r => r.role)).toEqual(['guitarist', 'producer'])
    expect(result.current.members[0].former).toBe(false)
  })

  it('refetch re-issues ListEntryPeople', async () => {
    let calls = 0
    const mockTransport = createRouterTransport(router => {
      router.service(EntryPersonService, {
        listEntryPeople: () => {
          calls += 1
          return { entryPeople: [] }
        },
      })
    })

    const { result } = renderHook(() => useArtistMembers('artist-1'), { wrapper: wrapper(mockTransport) })
    await waitFor(() => expect(result.current.isPending).toBe(false))
    expect(calls).toBe(1)

    result.current.refetch()
    await waitFor(() => expect(calls).toBe(2))
  })
})
