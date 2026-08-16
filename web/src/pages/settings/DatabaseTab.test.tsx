import { fireEvent, render, screen } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { useDatabaseInfo } from '../../hooks/useDatabaseInfo'
import { useDatabaseBackup } from '../../hooks/useDatabaseBackup'
import { useDatabaseRestore } from '../../hooks/useDatabaseRestore'
import { GetDatabaseInfoResponseSchema } from '../../gen/purser/database/v1/database_pb'
import { DatabaseTab } from './DatabaseTab'

// Page-level test: composition only, per ADR 0004 — databaseInfoFromProto's
// own conversion is tested in databaseFromProto.test.ts, formatBytes in
// databaseFormat.test.ts, and each hook's own RPC/streaming behavior in
// useDatabaseInfo.test.tsx/useDatabaseBackup.test.tsx/useDatabaseRestore.test.tsx.
// All three hooks are mocked directly so every state is reachable
// deterministically.
vi.mock('../../hooks/useDatabaseInfo')
const mockUseDatabaseInfo = vi.mocked(useDatabaseInfo)

vi.mock('../../hooks/useDatabaseBackup')
const mockUseDatabaseBackup = vi.mocked(useDatabaseBackup)

vi.mock('../../hooks/useDatabaseRestore')
const mockUseDatabaseRestore = vi.mocked(useDatabaseRestore)

function mockInfo(driver = 'badger', version = '4.2.0') {
  mockUseDatabaseInfo.mockReturnValue({
    isPending: false,
    isError: false,
    data: create(GetDatabaseInfoResponseSchema, {
      driver,
      version,
      storageSizeBytes: 2048n,
      collectionCounts: { person: 12n, tag: 340n },
    }),
    error: null,
  } as unknown as ReturnType<typeof useDatabaseInfo>)
}

const runBackup = vi.fn()
const runRestore = vi.fn()

// Overrides are loosely typed and the merged result cast wholesale, same
// as JobsTab.test.tsx's mockUseJobsList.mockReturnValue casts — the error
// states below only ever need a { message } shape, not a real ConnectError.
function mockActions(backupOverrides: Record<string, unknown> = {}, restoreOverrides: Record<string, unknown> = {}) {
  mockUseDatabaseBackup.mockReturnValue({
    phase: 'idle',
    bytesReceived: 0,
    error: undefined,
    runBackup,
    ...backupOverrides,
  } as unknown as ReturnType<typeof useDatabaseBackup>)
  mockUseDatabaseRestore.mockReturnValue({
    phase: 'idle',
    bytesSent: 0,
    totalBytes: 0,
    error: undefined,
    runRestore,
    ...restoreOverrides,
  } as unknown as ReturnType<typeof useDatabaseRestore>)
}

// runBackup/runRestore are shared vi.fn()s reused across every test in
// this file (mockActions re-attaches them to each mockReturnValue) — clear
// call history between tests so an earlier test's click doesn't leak into
// a later "not called" assertion.
afterEach(() => {
  vi.clearAllMocks()
})

describe('DatabaseTab', () => {
  it('renders nothing while database info is pending', () => {
    mockUseDatabaseInfo.mockReturnValue({
      isPending: true,
      isError: false,
      data: undefined,
      error: null,
    } as unknown as ReturnType<typeof useDatabaseInfo>)
    mockActions()

    const { container } = render(<DatabaseTab />)
    expect(container.textContent).toBe('')
  })

  it('shows an actionable error when GetDatabaseInfo fails', () => {
    mockUseDatabaseInfo.mockReturnValue({
      isPending: false,
      isError: true,
      data: undefined,
      error: { message: 'unavailable' },
    } as unknown as ReturnType<typeof useDatabaseInfo>)
    mockActions()

    render(<DatabaseTab />)
    expect(screen.getByRole('alert')).toHaveTextContent("Couldn't load database info (unavailable)")
  })

  it('renders info cards and per-collection bars from GetDatabaseInfo', () => {
    mockInfo('badger', '4.2.0')
    mockActions()

    render(<DatabaseTab />)

    expect(screen.getByText('badger')).toBeInTheDocument()
    expect(screen.getByText('4.2.0')).toBeInTheDocument()
    expect(screen.getByText('2.0 KB')).toBeInTheDocument()
    expect(screen.getByText('person')).toBeInTheDocument()
    expect(screen.getByText('12')).toBeInTheDocument()
    expect(screen.getByText('tag')).toBeInTheDocument()
    expect(screen.getByText('340')).toBeInTheDocument()
  })

  it('shows an empty state when there are no collections', () => {
    mockUseDatabaseInfo.mockReturnValue({
      isPending: false,
      isError: false,
      data: create(GetDatabaseInfoResponseSchema, { driver: 'sqlite', version: '3.45', storageSizeBytes: 0n, collectionCounts: {} }),
      error: null,
    } as unknown as ReturnType<typeof useDatabaseInfo>)
    mockActions()

    render(<DatabaseTab />)
    expect(screen.getByText('No collections found.')).toBeInTheDocument()
  })

  it('calls runBackup when the Backup button is clicked', () => {
    mockInfo()
    mockActions()

    render(<DatabaseTab />)
    fireEvent.click(screen.getByRole('button', { name: /Backup/ }))
    expect(runBackup).toHaveBeenCalledOnce()
  })

  it('shows backup download progress', () => {
    mockInfo()
    mockActions({ phase: 'downloading', bytesReceived: 1024 })

    render(<DatabaseTab />)
    expect(screen.getByText('Downloading… 1.0 KB')).toBeInTheDocument()
  })

  it('shows a backup error', () => {
    mockInfo()
    mockActions({ phase: 'error', error: { message: 'stream closed' } })

    render(<DatabaseTab />)
    expect(screen.getByRole('alert')).toHaveTextContent('Backup failed: stream closed')
  })

  it('prompts for confirmation and calls runRestore when a file is chosen and confirmed', () => {
    mockInfo()
    mockActions()
    vi.spyOn(window, 'confirm').mockReturnValue(true)

    render(<DatabaseTab />)
    const file = new File(['{"purser_backup_version":1}'], 'backup.jsonl', { type: 'application/x-ndjson' })
    fireEvent.change(screen.getByLabelText('Restore backup file'), { target: { files: [file] } })

    expect(window.confirm).toHaveBeenCalledOnce()
    expect(runRestore).toHaveBeenCalledWith(file)
  })

  it('does not call runRestore when the user cancels the confirmation', () => {
    mockInfo()
    mockActions()
    vi.spyOn(window, 'confirm').mockReturnValue(false)

    render(<DatabaseTab />)
    const file = new File(['data'], 'backup.jsonl')
    fireEvent.change(screen.getByLabelText('Restore backup file'), { target: { files: [file] } })

    expect(runRestore).not.toHaveBeenCalled()
  })

  it('shows restore upload progress, restart-waiting, and ready states', () => {
    mockInfo()
    mockActions({}, { phase: 'uploading', bytesSent: 512, totalBytes: 2048 })
    const { rerender } = render(<DatabaseTab />)
    expect(screen.getByText('Uploading… 512 B / 2.0 KB')).toBeInTheDocument()

    mockActions({}, { phase: 'restarting' })
    rerender(<DatabaseTab />)
    expect(screen.getByText('Restore applied — waiting for the server to restart…')).toBeInTheDocument()

    mockActions({}, { phase: 'ready' })
    rerender(<DatabaseTab />)
    expect(screen.getByText('Restore complete — server is back up.')).toBeInTheDocument()
  })

  it('shows a restore error', () => {
    mockInfo()
    mockActions({}, { phase: 'error', error: { message: 'invalid backup stream' } })

    render(<DatabaseTab />)
    expect(screen.getByText('Restore failed: invalid backup stream')).toBeInTheDocument()
  })

  it('disables both action buttons while a backup or restore is in flight', () => {
    mockInfo()
    mockActions({ phase: 'downloading' })

    render(<DatabaseTab />)
    expect(screen.getByRole('button', { name: /Backup/ })).toBeDisabled()
    expect(screen.getByRole('button', { name: /Restore/ })).toBeDisabled()
  })
})
