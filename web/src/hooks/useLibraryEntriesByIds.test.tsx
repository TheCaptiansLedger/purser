import { createRouterTransport } from '@connectrpc/connect'
import { ConnectError, Code } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { ReactNode } from 'react'
import { LibraryEntryService } from '../gen/purser/domain/v1/library_entry_pb'
import { useLibraryEntriesByIds } from './useLibraryEntriesByIds'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend — same createRouterTransport pattern as
// usePersonImages.test.tsx, the fan-out hook this one mirrors.
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

describe('useLibraryEntriesByIds', () => {
  it('resolves one LibraryEntry per id, keyed by id', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(LibraryEntryService, {
        getLibraryEntry: req => {
          if (req.id === 'entry-1') return { libraryEntry: { id: 'entry-1', name: 'Rumours' } }
          return { libraryEntry: { id: 'entry-2', name: 'Tusk' } }
        },
      })
    })

    const { result } = renderHook(() => useLibraryEntriesByIds(['entry-1', 'entry-2']), {
      wrapper: wrapper(mockTransport),
    })

    await waitFor(() => expect(result.current.isPending).toBe(false))
    expect(result.current.entriesById['entry-1'].name).toBe('Rumours')
    expect(result.current.entriesById['entry-2'].name).toBe('Tusk')
  })

  it('omits ids that fail to resolve rather than mapping them to an empty entry', async () => {
    const getLibraryEntrySpy = vi.fn(() => {
      throw new ConnectError('not found', Code.NotFound)
    })
    const mockTransport = createRouterTransport(router => {
      router.service(LibraryEntryService, { getLibraryEntry: getLibraryEntrySpy })
    })

    const { result } = renderHook(() => useLibraryEntriesByIds(['entry-3']), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(getLibraryEntrySpy).toHaveBeenCalledOnce())
    await waitFor(() => expect(result.current.isPending).toBe(false))
    expect(result.current.entriesById).toEqual({})
  })

  it('reports isPending false with no ids at all', () => {
    const mockTransport = createRouterTransport(() => {})

    const { result } = renderHook(() => useLibraryEntriesByIds([]), { wrapper: wrapper(mockTransport) })

    expect(result.current.isPending).toBe(false)
    expect(result.current.entriesById).toEqual({})
  })
})
