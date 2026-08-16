import type { Client } from '@connectrpc/connect'
import { ConnectError, createClient } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useCallback, useMemo, useState } from 'react'
import { DatabaseService } from '../gen/purser/database/v1/database_pb'

export type DatabaseRestorePhase = 'idle' | 'uploading' | 'restarting' | 'ready' | 'error'

export interface DatabaseRestoreState {
  phase: DatabaseRestorePhase
  bytesSent: number
  totalBytes: number
  error: ConnectError | undefined
}

export interface UseDatabaseRestore extends DatabaseRestoreState {
  runRestore: (file: File) => Promise<void>
}

// restoreChunkBytes bounds how much of the uploaded File each
// RestoreRequest carries. Independent of the backend's own
// backupChunkSize (internal/api/connect/database.go) — that constant
// bounds how Backup batches bytes on its *outgoing* stream, not what an
// uploader must match on Restore's incoming one.
const restoreChunkBytes = 64 * 1024

// restartPollIntervalMs/restartPollTimeoutMs bound how long
// useDatabaseRestore waits for the server to come back after a
// successful Restore — see docs/technical/database-backup-restore.md's
// restart orchestration and RestoreResponse's doc comment (a successful
// reply means the process is about to exit). Polling GetDatabaseInfo —
// the same RPC the tab's info cards already call — rather than a
// dedicated health endpoint, since this codebase has none.
const restartPollIntervalMs = 1000
const restartPollTimeoutMs = 60_000

// useDatabaseRestore calls DatabaseService.Restore (this codebase's first
// client-streaming RPC — see proto/purser/database/v1/database.proto)
// with the chosen backup File chunked into RestoreRequests, then waits
// for the server to restart before reporting done. Same raw-generated-
// client approach as useDatabaseBackup/useWatchJob — connect-query has no
// generated hook for streaming RPCs.
export function useDatabaseRestore(): UseDatabaseRestore {
  const transport = useTransport()
  const client = useMemo(() => createClient(DatabaseService, transport), [transport])

  const [state, setState] = useState<DatabaseRestoreState>({
    phase: 'idle',
    bytesSent: 0,
    totalBytes: 0,
    error: undefined,
  })

  const runRestore = useCallback(
    async (file: File) => {
      setState({ phase: 'uploading', bytesSent: 0, totalBytes: file.size, error: undefined })
      try {
        await client.restore(chunkFile(file, bytesSent => setState(prev => ({ ...prev, bytesSent }))))
        setState(prev => ({ ...prev, phase: 'restarting' }))
        await waitForRestart(client)
        setState(prev => ({ ...prev, phase: 'ready' }))
      } catch (err) {
        setState(prev => ({ ...prev, phase: 'error', error: ConnectError.from(err) }))
      }
    },
    [client],
  )

  return { ...state, runRestore }
}

// chunkFile yields file's bytes as a sequence of RestoreRequest-shaped
// objects, reporting cumulative bytes read after each slice. This is the
// caller-controlled pacing client-streaming RPCs need — Restore has no
// server-side progress to report back (it stages the whole upload before
// touching anything), so "bytes handed to the stream so far" is the only
// progress signal available, same as the pre-reset uploadWithProgress
// helper this replaces conceptually, not literally.
async function* chunkFile(file: File, onProgress: (bytesSent: number) => void): AsyncIterable<{ data: Uint8Array }> {
  let offset = 0
  while (offset < file.size) {
    const end = Math.min(offset + restoreChunkBytes, file.size)
    const buffer = await readAsArrayBuffer(file.slice(offset, end))
    yield { data: new Uint8Array(buffer) }
    offset = end
    onProgress(offset)
  }
}

// readAsArrayBuffer wraps FileReader in a Promise — used instead of the
// newer Blob.prototype.arrayBuffer() because jsdom's Blob/File polyfill
// (the test environment ADR 0004 runs against) doesn't implement it, and
// piping a jsdom Blob through fetch's Response silently truncates it too
// (Response's own Blob detection doesn't recognize jsdom's Blob class).
// FileReader is the one path that behaves the same under jsdom and real
// browsers, so tests exercise the same code a browser runs.
function readAsArrayBuffer(blob: Blob): Promise<ArrayBuffer> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(reader.result as ArrayBuffer)
    reader.onerror = () => reject(reader.error as DOMException)
    reader.readAsArrayBuffer(blob)
  })
}

// waitForRestart polls GetDatabaseInfo until it succeeds again — the
// server process that just applied the restore has exited, per
// RestoreResponse's contract, and an external supervisor is expected to
// bring it back up. Throws if the server hasn't come back within
// restartPollTimeoutMs.
async function waitForRestart(client: Client<typeof DatabaseService>) {
  const deadline = Date.now() + restartPollTimeoutMs
  for (;;) {
    try {
      await client.getDatabaseInfo({})
      return
    } catch {
      if (Date.now() >= deadline) {
        throw new Error('Server did not come back up after restore.')
      }
      await sleep(restartPollIntervalMs)
    }
  }
}

function sleep(ms: number): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, ms))
}
