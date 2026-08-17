import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import type { ReactNode } from 'react'
import { MonitorMode } from '../gen/purser/domain/v1/common_pb'
import { PersonService } from '../gen/purser/domain/v1/person_pb'
import { usePerson, useUpdatePersonMutation } from './usePerson'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend — same createRouterTransport pattern as useSettings.test.tsx.
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

describe('usePerson', () => {
  it('returns the person from a successful GetPerson call', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(PersonService, {
        getPerson: req => {
          expect(req.id).toBe('person-1')
          return { person: { id: 'person-1', name: 'Stevie Nicks' } }
        },
      })
    })

    const { result } = renderHook(() => usePerson('person-1'), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data?.person?.name).toBe('Stevie Nicks')
  })

  it('skips the call when id is empty', () => {
    const mockTransport = createRouterTransport(() => {})

    const { result } = renderHook(() => usePerson(''), { wrapper: wrapper(mockTransport) })

    expect(result.current.fetchStatus).toBe('idle')
  })
})

describe('useUpdatePersonMutation', () => {
  it('sends the given field mask and returns the updated person', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(PersonService, {
        updatePerson: req => {
          expect(req.updateMask?.paths).toEqual(['monitored', 'monitor_mode'])
          return { person: { id: 'person-1', name: 'Stevie Nicks', monitored: true, monitorMode: MonitorMode.ALL } }
        },
      })
    })

    const { result } = renderHook(() => useUpdatePersonMutation(), { wrapper: wrapper(mockTransport) })

    const response = await result.current.mutateAsync({
      person: { id: 'person-1', name: 'Stevie Nicks', monitored: true, monitorMode: MonitorMode.ALL },
      updateMask: { paths: ['monitored', 'monitor_mode'] },
    })

    expect(response.person?.monitored).toBe(true)
  })
})
