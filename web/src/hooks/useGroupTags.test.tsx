import { createRouterTransport } from '@connectrpc/connect'
import { ConnectError, Code } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import type { ReactNode } from 'react'
import { EntityType, TagScope } from '../gen/purser/domain/v1/common_pb'
import { TagService } from '../gen/purser/domain/v1/tag_pb'
import { TagAssignmentService } from '../gen/purser/domain/v1/tag_assignment_pb'
import { useGroupTags } from './useGroupTags'

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

describe('useGroupTags', () => {
  it('resolves each tag assignment to its Tag', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(TagAssignmentService, {
        listTagAssignments: request => {
          expect(request.entityType).toBe(EntityType.GROUP)
          expect(request.entityId).toBe('group-1')
          return {
            tagAssignments: [
              { tagId: 'tag-1', entityType: EntityType.GROUP, entityId: 'group-1' },
              { tagId: 'tag-2', entityType: EntityType.GROUP, entityId: 'group-1' },
            ],
          }
        },
      })
      router.service(TagService, {
        getTag: request => {
          if (request.id === 'tag-1') {
            return { tag: { id: 'tag-1', key: 'genre', value: 'Rock', scope: TagScope.METADATA } }
          }
          return { tag: { id: 'tag-2', key: 'mood', value: 'Energetic', scope: TagScope.METADATA } }
        },
      })
    })

    const { result } = renderHook(() => useGroupTags('group-1'), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current.isPending).toBe(false))
    expect(result.current.tags).toEqual([
      { id: 'tag-1', key: 'genre', value: 'Rock' },
      { id: 'tag-2', key: 'mood', value: 'Energetic' },
    ])
  })

  it('resolves to an empty array when the Group has zero tag assignments', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(TagAssignmentService, { listTagAssignments: () => ({ tagAssignments: [] }) })
    })

    const { result } = renderHook(() => useGroupTags('group-1'), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current.isPending).toBe(false))
    expect(result.current.tags).toEqual([])
  })

  it('omits an assignment whose Tag fails to resolve rather than erroring the whole hook', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(TagAssignmentService, {
        listTagAssignments: () => ({ tagAssignments: [{ tagId: 'tag-1', entityType: EntityType.GROUP, entityId: 'group-1' }] }),
      })
      router.service(TagService, {
        getTag: () => {
          throw new ConnectError('not found', Code.NotFound)
        },
      })
    })

    const { result } = renderHook(() => useGroupTags('group-1'), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current.isPending).toBe(false))
    expect(result.current.tags).toEqual([])
  })
})
