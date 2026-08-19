import { createRouterTransport } from '@connectrpc/connect'
import { ConnectError, Code } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { ReactNode } from 'react'
import { GroupService } from '../gen/purser/domain/v1/group_pb'
import { useGroupsByIds } from './useGroupsByIds'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend — same pattern as useLibraryEntriesByIds.test.tsx, the
// fan-out hook this one mirrors.
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

describe('useGroupsByIds', () => {
  it('resolves one Group per id, keyed by id', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(GroupService, {
        getGroup: req => {
          if (req.id === 'group-1') return { group: { id: 'group-1', title: 'Rumours' } }
          return { group: { id: 'group-2', title: 'Tusk' } }
        },
      })
    })

    const { result } = renderHook(() => useGroupsByIds(['group-1', 'group-2']), {
      wrapper: wrapper(mockTransport),
    })

    await waitFor(() => expect(result.current.isPending).toBe(false))
    expect(result.current.groupsById['group-1'].title).toBe('Rumours')
    expect(result.current.groupsById['group-2'].title).toBe('Tusk')
  })

  it('omits ids that fail to resolve rather than mapping them to an empty group', async () => {
    const getGroupSpy = vi.fn(() => {
      throw new ConnectError('not found', Code.NotFound)
    })
    const mockTransport = createRouterTransport(router => {
      router.service(GroupService, { getGroup: getGroupSpy })
    })

    const { result } = renderHook(() => useGroupsByIds(['group-3']), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(getGroupSpy).toHaveBeenCalledOnce())
    await waitFor(() => expect(result.current.isPending).toBe(false))
    expect(result.current.groupsById).toEqual({})
  })

  it('reports isPending false with no ids at all', () => {
    const mockTransport = createRouterTransport(() => {})

    const { result } = renderHook(() => useGroupsByIds([]), { wrapper: wrapper(mockTransport) })

    expect(result.current.isPending).toBe(false)
    expect(result.current.groupsById).toEqual({})
  })
})
