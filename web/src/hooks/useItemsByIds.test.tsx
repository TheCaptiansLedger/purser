import { createRouterTransport } from '@connectrpc/connect'
import { ConnectError, Code } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { ReactNode } from 'react'
import { ItemService } from '../gen/purser/domain/v1/item_pb'
import { useItemsByIds } from './useItemsByIds'

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

describe('useItemsByIds', () => {
  it('resolves one Item per id, keyed by id', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(ItemService, {
        getItem: req => {
          if (req.id === 'item-1') return { item: { id: 'item-1', title: 'Dreams' } }
          return { item: { id: 'item-2', title: 'Sara' } }
        },
      })
    })

    const { result } = renderHook(() => useItemsByIds(['item-1', 'item-2']), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current.isPending).toBe(false))
    expect(result.current.itemsById['item-1'].title).toBe('Dreams')
    expect(result.current.itemsById['item-2'].title).toBe('Sara')
  })

  it('omits ids that fail to resolve rather than mapping them to an empty item', async () => {
    const getItemSpy = vi.fn(() => {
      throw new ConnectError('not found', Code.NotFound)
    })
    const mockTransport = createRouterTransport(router => {
      router.service(ItemService, { getItem: getItemSpy })
    })

    const { result } = renderHook(() => useItemsByIds(['item-3']), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(getItemSpy).toHaveBeenCalledOnce())
    await waitFor(() => expect(result.current.isPending).toBe(false))
    expect(result.current.itemsById).toEqual({})
  })

  it('reports isPending false with no ids at all', () => {
    const mockTransport = createRouterTransport(() => {})

    const { result } = renderHook(() => useItemsByIds([]), { wrapper: wrapper(mockTransport) })

    expect(result.current.isPending).toBe(false)
    expect(result.current.itemsById).toEqual({})
  })
})
