import { render, screen } from '@testing-library/react'
import { MemoryRouter, Navigate, Route, Routes } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { SettingsLayout } from './SettingsLayout'
import { ConfigTab } from './ConfigTab'
import { JobsTab } from './JobsTab'
import { useSettings } from '../../hooks/useSettings'

// SettingsLayout is a page — this tests that it composes the tab nav and
// routes the outlet to the right child, not the internal behavior of
// NavLink itself (ADR 0004's page-level testing rule), nor ConfigTab's
// own data-fetching (ConfigTab.test.tsx). useSettings is mocked to its
// pending state purely so ConfigTab renders deterministically (nothing)
// without a real Connect round trip — SettingsLayout's job is proven by
// which tab is active/highlighted, not by ConfigTab's content.
vi.mock('../../hooks/useSettings')
const mockUseSettings = vi.mocked(useSettings)

function renderAt(path: string) {
  render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/settings" element={<SettingsLayout />}>
          <Route index element={<Navigate to="config" replace />} />
          <Route path="config" element={<ConfigTab />} />
          <Route path="jobs" element={<JobsTab />} />
        </Route>
      </Routes>
    </MemoryRouter>,
  )
}

describe('SettingsLayout', () => {
  beforeEach(() => {
    mockUseSettings.mockReturnValue({
      isPending: true,
      isError: false,
      data: undefined,
      error: null,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useSettings>)
  })

  it('renders all four tab links', () => {
    renderAt('/settings/config')
    expect(screen.getByRole('link', { name: 'Config' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Jobs' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Database' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Cache' })).toBeInTheDocument()
  })

  it('renders the Config tab at /settings/config', () => {
    renderAt('/settings/config')
    expect(mockUseSettings).toHaveBeenCalled()
    expect(screen.getByRole('link', { name: 'Config' })).toHaveClass('border-text')
    expect(screen.getByRole('link', { name: 'Jobs' })).not.toHaveClass('border-text')
  })

  it('renders the Jobs tab content at /settings/jobs', () => {
    renderAt('/settings/jobs')
    expect(screen.getByText('Jobs — coming soon.')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Jobs' })).toHaveClass('border-text')
  })

  it('redirects the bare /settings index to the Config tab', () => {
    renderAt('/settings')
    expect(screen.getByRole('link', { name: 'Config' })).toHaveClass('border-text')
    expect(mockUseSettings).toHaveBeenCalled()
  })
})
