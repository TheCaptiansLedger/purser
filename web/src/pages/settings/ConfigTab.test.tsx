import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { create } from '@bufbuild/protobuf'
import type { MessageInitShape } from '@bufbuild/protobuf'
import { useResetSettingMutation, useSettings, useUpdateSettingsMutation } from '../../hooks/useSettings'
import { SettingLockReason, SettingSchema, SettingSource } from '../../gen/purser/settings/v1/settings_pb'
import { ConfigTab } from './ConfigTab'

// Page-level test: composition only, per ADR 0004 — SettingsCard's own
// field-editing/lock-display behavior is tested in SettingsCard.test.tsx,
// and useSettings/useUpdateSettingsMutation/useResetSettingMutation's
// wire behavior in useSettings.test.tsx. Here the hooks are mocked
// directly so every state is reachable deterministically.
vi.mock('../../hooks/useSettings')
const mockUseSettings = vi.mocked(useSettings)
const mockUseUpdateSettingsMutation = vi.mocked(useUpdateSettingsMutation)
const mockUseResetSettingMutation = vi.mocked(useResetSettingMutation)

function proto(overrides: MessageInitShape<typeof SettingSchema>) {
  return create(SettingSchema, {
    key: 'modules.movies.enabled',
    value: 'false',
    source: SettingSource.DEFAULT,
    locked: false,
    lockReason: SettingLockReason.UNSPECIFIED,
    secret: false,
    ...overrides,
  })
}

function stubMutations() {
  mockUseUpdateSettingsMutation.mockReturnValue({
    mutateAsync: vi.fn().mockResolvedValue(undefined),
    isPending: false,
  } as unknown as ReturnType<typeof useUpdateSettingsMutation>)
  mockUseResetSettingMutation.mockReturnValue({
    mutate: vi.fn(),
    isPending: false,
    variables: undefined,
  } as unknown as ReturnType<typeof useResetSettingMutation>)
}

describe('ConfigTab', () => {
  it('renders nothing while pending', () => {
    mockUseSettings.mockReturnValue({ isPending: true, isError: false, data: undefined, error: null, refetch: vi.fn() } as unknown as ReturnType<
      typeof useSettings
    >)
    stubMutations()

    const { container } = render(<ConfigTab />)
    expect(container.textContent).toBe('')
  })

  it('shows an actionable error when GetSettings fails', () => {
    mockUseSettings.mockReturnValue({
      isPending: false,
      isError: true,
      data: undefined,
      error: { message: 'unavailable' },
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useSettings>)
    stubMutations()

    render(<ConfigTab />)
    expect(screen.getByRole('alert')).toHaveTextContent("Couldn't load settings (unavailable)")
  })

  it('renders only categories that have settings, as cards in category order', () => {
    mockUseSettings.mockReturnValue({
      isPending: false,
      isError: false,
      data: {
        settings: [
          proto({ key: 'modules.movies.enabled' }),
          proto({ key: 'database.driver', value: '"badger"', locked: true, lockReason: SettingLockReason.BOOTSTRAP }),
        ],
      },
      error: null,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useSettings>)
    stubMutations()

    render(<ConfigTab />)

    const headings = screen.getAllByRole('heading', { level: 2 }).map(h => h.textContent)
    expect(headings).toEqual(['Database', 'Movies'])
  })

  it('saves a dirty field via updateSettings and refetches on success', async () => {
    const refetch = vi.fn()
    const mutateAsync = vi.fn().mockResolvedValue(undefined)
    mockUseSettings.mockReturnValue({
      isPending: false,
      isError: false,
      data: { settings: [proto({ key: 'modules.movies.enabled', value: 'false' })] },
      error: null,
      refetch,
    } as unknown as ReturnType<typeof useSettings>)
    mockUseUpdateSettingsMutation.mockReturnValue({ mutateAsync, isPending: false } as unknown as ReturnType<
      typeof useUpdateSettingsMutation
    >)
    mockUseResetSettingMutation.mockReturnValue({ mutate: vi.fn(), isPending: false, variables: undefined } as unknown as ReturnType<
      typeof useResetSettingMutation
    >)

    render(<ConfigTab />)

    fireEvent.click(screen.getByRole('switch', { name: 'Enabled' }))
    fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))

    await waitFor(() =>
      expect(mutateAsync).toHaveBeenCalledWith({
        values: { 'modules.movies.enabled': 'true' },
        updateMask: ['modules.movies.enabled'],
      }),
    )
    await waitFor(() => expect(refetch).toHaveBeenCalled())
  })
})
