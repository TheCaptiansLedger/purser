import { useRef } from 'react'
import type { ReactNode } from 'react'
import { AlertCircle, CheckCircle2, Download, Loader2, RefreshCw, Upload } from 'lucide-react'
import { useDatabaseInfo } from '../../hooks/useDatabaseInfo'
import { useDatabaseBackup } from '../../hooks/useDatabaseBackup'
import type { UseDatabaseBackup } from '../../hooks/useDatabaseBackup'
import { useDatabaseRestore } from '../../hooks/useDatabaseRestore'
import type { UseDatabaseRestore } from '../../hooks/useDatabaseRestore'
import { databaseInfoFromProto } from './databaseFromProto'
import { formatBytes } from './databaseFormat'
import type { DatabaseInfo } from '../../types'

// DatabaseTab is #613's Database tab: read-only info cards
// (driver/version/storage size), per-collection record-count bars, and
// backup/restore actions with progress and post-restore restart-polling —
// full UX parity with the pre-reset DatabasePage (prior art for shape
// only, not resurrected code), rebuilt on #612's Connect
// server-/client-streaming RPCs and this app's design-token system
// instead of the pre-reset REST endpoints and raw Tailwind. See
// docs/technical/database-backup-restore.md.
export function DatabaseTab() {
  const infoQuery = useDatabaseInfo()
  const backup = useDatabaseBackup()
  const restore = useDatabaseRestore()
  const fileInputRef = useRef<HTMLInputElement>(null)

  const busy = backup.phase === 'downloading' || restore.phase === 'uploading' || restore.phase === 'restarting'

  function handleRestoreClick() {
    if (!busy) fileInputRef.current?.click()
  }

  function handleFileChange(event: React.ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (!file) return
    // Restore is destructive (every existing record is replaced) and
    // makes the server restart — a native confirm is enough friction for
    // a single, rarely-used destructive action; no dedicated dialog
    // component exists in this codebase yet to justify building one for
    // this alone.
    const confirmed = window.confirm(
      "Restoring replaces every record in the database with this backup file's contents, and the server will restart. This can't be undone. Continue?",
    )
    if (!confirmed) return
    void restore.runRestore(file)
  }

  // Doherty threshold — see docs/design/ux-principles.md#feedback--system-status.
  // A local Connect round trip resolves well under 400ms; a loading
  // indicator here would read as slower, not more informative.
  if (infoQuery.isPending) {
    return null
  }

  if (infoQuery.isError) {
    return (
      <p className="text-status-failure text-body" role="alert">
        Couldn't load database info ({infoQuery.error.message}).
      </p>
    )
  }

  const info = databaseInfoFromProto(infoQuery.data)

  return (
    <div className="flex flex-col gap-4">
      <InfoCards info={info} />
      <CollectionsCard collectionCounts={info.collectionCounts} />
      <ActionsCard
        backup={backup}
        restore={restore}
        busy={busy}
        onBackupClick={() => void backup.runBackup()}
        onRestoreClick={handleRestoreClick}
      />
      <input
        ref={fileInputRef}
        type="file"
        accept=".jsonl,.json,.txt"
        className="hidden"
        onChange={handleFileChange}
        aria-label="Restore backup file"
      />
    </div>
  )
}

function InfoCards({ info }: { info: DatabaseInfo }) {
  const stats: { label: string; value: string }[] = [
    { label: 'Driver', value: info.driver },
    { label: 'Version', value: info.version },
    { label: 'Storage size', value: formatBytes(info.storageSizeBytes) },
  ]
  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
      {stats.map(stat => (
        <section key={stat.label} className="bg-surface border border-border rounded-lg p-5 flex flex-col gap-1">
          <span className="text-label text-text-secondary">{stat.label}</span>
          <span className="text-title-md text-text font-mono">{stat.value}</span>
        </section>
      ))}
    </div>
  )
}

function CollectionsCard({ collectionCounts }: { collectionCounts: Record<string, number> }) {
  const entries = Object.entries(collectionCounts).sort(([a], [b]) => a.localeCompare(b))
  const max = Math.max(...entries.map(([, count]) => count), 1)

  return (
    <section className="bg-surface border border-border rounded-lg p-5 flex flex-col gap-4">
      <h2 className="text-title-md text-text">Collections</h2>
      {entries.length === 0 ? (
        <p className="text-body text-text-secondary">No collections found.</p>
      ) : (
        <div className="flex flex-col gap-2.5">
          {entries.map(([collection, count]) => (
            <div key={collection} className="flex items-center gap-3">
              <span className="text-label text-text-secondary font-mono w-44 truncate shrink-0">{collection}</span>
              <div className="flex-1 h-1.5 rounded-full bg-surface-raised overflow-hidden">
                <div className="h-full rounded-full bg-accent-system" style={{ width: `${(count / max) * 100}%` }} />
              </div>
              <span className="text-label text-text-secondary font-mono w-12 text-right shrink-0">{count.toLocaleString()}</span>
            </div>
          ))}
        </div>
      )}
    </section>
  )
}

interface ActionsCardProps {
  backup: UseDatabaseBackup
  restore: UseDatabaseRestore
  busy: boolean
  onBackupClick: () => void
  onRestoreClick: () => void
}

function ActionsCard({ backup, restore, busy, onBackupClick, onRestoreClick }: ActionsCardProps) {
  return (
    <section className="bg-surface border border-border rounded-lg p-5 flex flex-col gap-4">
      <h2 className="text-title-md text-text">Actions</h2>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <button
          type="button"
          onClick={onBackupClick}
          disabled={busy}
          className="rounded-lg border border-border bg-surface-raised px-4 py-4 flex items-center gap-3 text-left hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {backup.phase === 'downloading' ? (
            <Loader2 size={18} className="text-accent-system animate-spin shrink-0" />
          ) : (
            <Download size={18} className="text-accent-system shrink-0" />
          )}
          <div>
            <div className="text-body font-medium text-text">Backup</div>
            <div className="text-label text-text-secondary">Download the database as a portable backup file</div>
          </div>
        </button>
        <button
          type="button"
          onClick={onRestoreClick}
          disabled={busy}
          className="rounded-lg border border-border bg-surface-raised px-4 py-4 flex items-center gap-3 text-left hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {restore.phase === 'uploading' || restore.phase === 'restarting' ? (
            <Loader2 size={18} className="text-accent-system animate-spin shrink-0" />
          ) : (
            <Upload size={18} className="text-accent-system shrink-0" />
          )}
          <div>
            <div className="text-body font-medium text-text">Restore</div>
            <div className="text-label text-text-secondary">Replace the database with a backup file</div>
          </div>
        </button>
      </div>

      {backup.phase === 'downloading' && (
        <StatusRow icon={<Loader2 size={14} className="animate-spin" />} text={`Downloading… ${formatBytes(backup.bytesReceived)}`} />
      )}
      {backup.phase === 'error' && (
        <StatusRow icon={<AlertCircle size={14} />} text={`Backup failed: ${backup.error?.message}`} tone="failure" />
      )}

      {restore.phase === 'uploading' && (
        <StatusRow
          icon={<Loader2 size={14} className="animate-spin" />}
          text={`Uploading… ${formatBytes(restore.bytesSent)} / ${formatBytes(restore.totalBytes)}`}
        />
      )}
      {restore.phase === 'restarting' && (
        <StatusRow icon={<RefreshCw size={14} className="animate-spin" />} text="Restore applied — waiting for the server to restart…" />
      )}
      {restore.phase === 'ready' && (
        <StatusRow icon={<CheckCircle2 size={14} />} text="Restore complete — server is back up." tone="success" />
      )}
      {restore.phase === 'error' && (
        <StatusRow icon={<AlertCircle size={14} />} text={`Restore failed: ${restore.error?.message}`} tone="failure" />
      )}
    </section>
  )
}

function StatusRow({ icon, text, tone = 'neutral' }: { icon: ReactNode; text: string; tone?: 'neutral' | 'success' | 'failure' }) {
  const toneClass = tone === 'success' ? 'text-status-success' : tone === 'failure' ? 'text-status-failure' : 'text-text-secondary'
  return (
    <div className={`flex items-center gap-2 text-label ${toneClass}`} role={tone === 'failure' ? 'alert' : undefined}>
      {icon}
      <span>{text}</span>
    </div>
  )
}
