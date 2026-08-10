import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import type { ReactNode } from 'react'
import { SettingsService } from '../gen/purser/settings/v1/settings_pb'
import { useResetSettingMutation, useSettings, useUpdateSettingsMutation } from './useSettings'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend — same createRouterTransport pattern as useJobs.test.tsx.
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

describe('useSettings', () => {
  it('returns settings from a successful GetSettings call', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(SettingsService, {
        getSettings: () => ({
          settings: [{ key: 'media.path', value: '"/data/media"', source: 1, locked: false, lockReason: 0, secret: false }],
        }),
      })
    })

    const { result } = renderHook(() => useSettings(), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data?.settings).toHaveLength(1)
    expect(result.current.data?.settings[0].key).toBe('media.path')
  })

  it('surfaces a ConnectError when GetSettings fails', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(SettingsService, {
        getSettings: () => {
          throw new Error('unavailable')
        },
      })
    })

    const { result } = renderHook(() => useSettings(), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current.isError).toBe(true))
    expect(result.current.error).toBeTruthy()
  })
})

describe('useUpdateSettingsMutation', () => {
  it('sends the values/update_mask the caller passes and returns the response', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(SettingsService, {
        updateSettings: req => {
          expect(req.updateMask).toEqual(['media.path'])
          expect(req.values).toEqual({ 'media.path': '"/new/path"' })
          return {
            settings: [{ key: 'media.path', value: '"/new/path"', source: 4, locked: false, lockReason: 0, secret: false }],
          }
        },
      })
    })

    const { result } = renderHook(() => useUpdateSettingsMutation(), { wrapper: wrapper(mockTransport) })

    result.current.mutate({ values: { 'media.path': '"/new/path"' }, updateMask: ['media.path'] })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data?.settings[0].value).toBe('"/new/path"')
  })
})

describe('useResetSettingMutation', () => {
  it('sends the key the caller passes and returns the response', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(SettingsService, {
        resetSetting: req => {
          expect(req.key).toBe('media.path')
          return { setting: { key: 'media.path', value: '"/data/media"', source: 1, locked: false, lockReason: 0, secret: false } }
        },
      })
    })

    const { result } = renderHook(() => useResetSettingMutation(), { wrapper: wrapper(mockTransport) })

    result.current.mutate({ key: 'media.path' })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data?.setting?.value).toBe('"/data/media"')
  })
})
