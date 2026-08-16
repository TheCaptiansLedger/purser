import { ConnectError, createClient } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useCallback, useMemo, useState } from 'react'
import { DatabaseService } from '../gen/purser/database/v1/database_pb'

export type DatabaseBackupPhase = 'idle' | 'downloading' | 'done' | 'error'

export interface DatabaseBackupState {
  phase: DatabaseBackupPhase
  bytesReceived: number
  error: ConnectError | undefined
}

export interface UseDatabaseBackup extends DatabaseBackupState {
  runBackup: () => Promise<void>
}

// useDatabaseBackup calls DatabaseService.Backup (server-streaming — see
// proto/purser/database/v1/database.proto), assembling the streamed
// BackupChunks into a Blob and triggering a same-origin download via a
// throwaway anchor element. connect-query has no generated hook for
// streaming RPCs, so this reaches for the raw generated client the same
// way useWatchJob.ts does, sharing the TransportProvider transport every
// connect-query hook already uses. There's no REST download URL to
// navigate to anymore — the pre-reset app's
// `window.location.href = '/api/v1/database/backup'` doesn't apply once
// Backup is a Connect RPC rather than a plain HTTP GET, so the browser
// only sees a download once every chunk has arrived and been assembled.
export function useDatabaseBackup(): UseDatabaseBackup {
  const transport = useTransport()
  const client = useMemo(() => createClient(DatabaseService, transport), [transport])

  const [state, setState] = useState<DatabaseBackupState>({
    phase: 'idle',
    bytesReceived: 0,
    error: undefined,
  })

  const runBackup = useCallback(async () => {
    setState({ phase: 'downloading', bytesReceived: 0, error: undefined })
    try {
      const chunks: BlobPart[] = []
      let bytesReceived = 0
      for await (const chunk of client.backup({})) {
        // chunk.data is Uint8Array<ArrayBufferLike> (protobuf-es's typing
        // doesn't guarantee a plain, non-shared ArrayBuffer backing);
        // Blob's BlobPart type requires ArrayBufferView<ArrayBuffer>, so
        // copy into a fresh Uint8Array<ArrayBuffer> here rather than
        // pushing chunk.data directly.
        chunks.push(new Uint8Array(chunk.data))
        bytesReceived += chunk.data.length
        setState(prev => ({ ...prev, bytesReceived }))
      }
      downloadBlob(new Blob(chunks, { type: 'application/x-ndjson' }))
      setState(prev => ({ ...prev, phase: 'done' }))
    } catch (err) {
      setState(prev => ({ ...prev, phase: 'error', error: ConnectError.from(err) }))
    }
  }, [client])

  return { ...state, runBackup }
}

// downloadBlob triggers a browser download the same way an <a download>
// click would for a same-origin file — the only way to hand the browser
// a Blob assembled from a streamed RPC response, since there's no URL
// left to navigate to directly.
function downloadBlob(blob: Blob) {
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = `purser-backup-${new Date().toISOString().replace(/[:.]/g, '-')}.jsonl`
  document.body.appendChild(anchor)
  anchor.click()
  anchor.remove()
  URL.revokeObjectURL(url)
}
