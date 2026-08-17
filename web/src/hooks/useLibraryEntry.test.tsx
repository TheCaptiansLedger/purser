import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import type { ReactNode } from 'react'
import { MonitorMode } from '../gen/purser/domain/v1/common_pb'
import { LibraryEntryService } from '../gen/purser/domain/v1/library_entry_pb'
import { useLibraryEntry, useUpdateLibraryEntryMutation } from './useLibraryEntry'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend — same createRouterTransport pattern as usePerson.test.tsx.
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

describe('useLibraryEntry', () => {
  it('returns the library entry from a successful GetLibraryEntry call', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(LibraryEntryService, {
        getLibraryEntry: req => {
          expect(req.id).toBe('artist-1')
          return { libraryEntry: { id: 'artist-1', name: 'Fleetwood Mac' } }
        },
      })
    })

    const { result } = renderHook(() => useLibraryEntry('artist-1'), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data?.libraryEntry?.name).toBe('Fleetwood Mac')
  })

  it('skips the call when id is empty', () => {
    const mockTransport = createRouterTransport(() => {})

    const { result } = renderHook(() => useLibraryEntry(''), { wrapper: wrapper(mockTransport) })

    expect(result.current.fetchStatus).toBe('idle')
  })
})

describe('useUpdateLibraryEntryMutation', () => {
  it('sends the given field mask and returns the updated library entry', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(LibraryEntryService, {
        updateLibraryEntry: req => {
          expect(req.updateMask?.paths).toEqual(['monitored', 'monitor_mode'])
          return { libraryEntry: { id: 'artist-1', name: 'Fleetwood Mac', monitored: true, monitorMode: MonitorMode.ALL } }
        },
      })
    })

    const { result } = renderHook(() => useUpdateLibraryEntryMutation(), { wrapper: wrapper(mockTransport) })

    const response = await result.current.mutateAsync({
      libraryEntry: { id: 'artist-1', monitored: true, monitorMode: MonitorMode.ALL },
      updateMask: { paths: ['monitored', 'monitor_mode'] },
    })

    expect(response.libraryEntry?.monitored).toBe(true)
  })
})
