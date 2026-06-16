import { useRef, useState, useEffect } from 'react'
import { Download, Upload, Loader2, CheckCircle2, AlertCircle, RefreshCw } from 'lucide-react'
import { useDBStats, uploadWithProgress } from '../../api/database'
import type { RestoreResult } from '../../api/database'

type Phase = 'idle' | 'uploading' | 'processing' | 'restarting' | 'ready' | 'error'

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

function ProgressBar({ pct, accent }: { pct: number; accent: string }) {
  return (
    <div className="w-full h-1.5 rounded-full bg-white/10 overflow-hidden">
      <div
        className="h-full rounded-full transition-all duration-200"
        style={{ width: `${pct}%`, background: accent }}
      />
    </div>
  )
}

function RestoreStats({ result }: { result: RestoreResult }) {
  const max = Math.max(...result.tables.map(t => t.rows), 1)
  return (
    <div className="mt-4 rounded-xl border border-emerald-500/20 bg-emerald-500/5 overflow-hidden">
      <div className="flex items-center gap-2 px-4 py-3 border-b border-emerald-500/10">
        <CheckCircle2 size={15} className="text-emerald-400 shrink-0" />
        <span className="text-sm font-medium text-emerald-300">
          Restore complete — {result.total_rows.toLocaleString()} rows restored
        </span>
      </div>
      <div className="px-4 py-3 space-y-2">
        {result.tables.map(t => (
          <div key={t.name} className="flex items-center gap-3">
            <span className="text-xs text-white/40 font-mono w-44 truncate shrink-0">{t.name}</span>
            <div className="flex-1 h-1 rounded-full bg-white/5 overflow-hidden">
              <div
                className="h-full rounded-full"
                style={{ width: `${(t.rows / max) * 100}%`, background: 'rgba(52,211,153,0.4)' }}
              />
            </div>
            <span className="text-xs text-white/50 font-mono w-10 text-right shrink-0">
              {t.rows.toLocaleString()}
            </span>
          </div>
        ))}
      </div>
    </div>
  )
}

export function DatabasePage() {
  const { data: stats, isLoading } = useDBStats()
  const fileRef = useRef<HTMLInputElement>(null)

  const [phase, setPhase]           = useState<Phase>('idle')
  const [uploadPct, setUploadPct]   = useState(0)
  const [restoreResult, setResult]  = useState<RestoreResult | null>(null)
  const [errorMsg, setErrorMsg]     = useState('')

  // After a successful restore, poll until the server is back up.
  useEffect(() => {
    if (phase !== 'restarting') return
    const id = setInterval(async () => {
      try {
        const r = await fetch('/api/v1/config', { signal: AbortSignal.timeout(2000) })
        if (r.ok) {
          clearInterval(id)
          setPhase('ready')
        }
      } catch { /* still down */ }
    }, 2000)
    return () => clearInterval(id)
  }, [phase])

  function handleBackup() {
    window.location.href = '/api/v1/database/backup'
  }

  async function handleFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return
    if (fileRef.current) fileRef.current.value = ''

    setPhase('uploading')
    setUploadPct(0)
    setResult(null)
    setErrorMsg('')

    try {
      const result = await uploadWithProgress(file, (pct) => {
        setUploadPct(pct)
        if (pct >= 99) setPhase('processing')
      })
      setResult(result)
      setPhase('restarting')
    } catch (err: any) {
      setPhase('error')
      setErrorMsg(err.message ?? 'Restore failed')
    }
  }

  const busy = phase === 'uploading' || phase === 'processing' || phase === 'restarting'
  const maxRows = Math.max(...(stats?.tables.map(t => t.rows) ?? []), 1)

  return (
    <div className="px-8 py-10 max-w-2xl">
      <div className="mb-10">
        <h1 className="text-2xl font-semibold text-white">Database</h1>
        <p className="text-white/40 text-sm mt-1">Backup and restore your Purser database</p>
      </div>

      {/* Actions */}
      <section className="mb-8">
        <h2 className="text-xs font-semibold uppercase tracking-widest text-white/30 mb-3">Actions</h2>
        <div className="grid grid-cols-2 gap-3">
          <button
            onClick={handleBackup}
            disabled={busy}
            className="rounded-xl border border-white/5 bg-white/2 px-5 py-5 flex flex-col gap-3
              hover:bg-white/5 hover:border-white/10 transition-all duration-150 text-left group
              disabled:opacity-40 disabled:cursor-not-allowed"
          >
            <div className="w-10 h-10 rounded-lg bg-blue-500/10 flex items-center justify-center">
              <Download size={18} className="text-blue-400" strokeWidth={2} />
            </div>
            <div>
              <div className="text-sm font-medium text-white/80 group-hover:text-white transition-colors">Backup</div>
              <div className="text-xs text-white/35 mt-0.5">Download database as SQL</div>
            </div>
          </button>

          <button
            onClick={() => !busy && fileRef.current?.click()}
            disabled={busy}
            className="rounded-xl border border-white/5 bg-white/2 px-5 py-5 flex flex-col gap-3
              hover:bg-white/5 hover:border-white/10 transition-all duration-150 text-left group
              disabled:opacity-40 disabled:cursor-not-allowed"
          >
            <div className="w-10 h-10 rounded-lg bg-emerald-500/10 flex items-center justify-center">
              {busy
                ? <Loader2 size={18} className="text-emerald-400 animate-spin" />
                : <Upload size={18} className="text-emerald-400" strokeWidth={2} />
              }
            </div>
            <div>
              <div className="text-sm font-medium text-white/80 group-hover:text-white transition-colors">Restore</div>
              <div className="text-xs text-white/35 mt-0.5">Import a SQL backup file</div>
            </div>
          </button>
        </div>

        <input ref={fileRef} type="file" accept=".sql" className="hidden" onChange={handleFileChange} />

        {/* Progress / status feedback */}
        {phase === 'uploading' && (
          <div className="mt-3 rounded-xl border border-white/5 bg-white/2 px-4 py-3 space-y-2">
            <div className="flex justify-between text-xs text-white/50 font-mono">
              <span>Uploading…</span>
              <span>{uploadPct}%</span>
            </div>
            <ProgressBar pct={uploadPct} accent="rgba(52,211,153,0.7)" />
          </div>
        )}

        {phase === 'processing' && (
          <div className="mt-3 rounded-xl border border-white/5 bg-white/2 px-4 py-3 space-y-2">
            <div className="flex justify-between text-xs text-white/50 font-mono">
              <span>Processing SQL…</span>
              <span>100%</span>
            </div>
            <ProgressBar pct={100} accent="rgba(52,211,153,0.7)" />
          </div>
        )}

        {(phase === 'restarting' || phase === 'ready') && restoreResult && (
          <>
            <RestoreStats result={restoreResult} />
            {phase === 'restarting' ? (
              <div className="mt-3 flex items-center gap-2 px-4 py-3 rounded-lg bg-white/3 border border-white/5">
                <RefreshCw size={13} className="text-white/40 animate-spin shrink-0" />
                <span className="text-xs text-white/40 font-mono">Waiting for server to restart…</span>
              </div>
            ) : (
              <div className="mt-3 flex items-center gap-2 px-4 py-3 rounded-lg bg-white/3 border border-white/8">
                <CheckCircle2 size={13} className="text-emerald-400/60 shrink-0" />
                <span className="text-xs text-white/40 font-mono">Server is ready</span>
              </div>
            )}
          </>
        )}

        {phase === 'error' && (
          <div className="mt-3 space-y-2">
            <div className="flex items-start gap-2 px-4 py-3 rounded-lg bg-red-500/10 border border-red-500/20">
              <AlertCircle size={15} className="text-red-400 shrink-0 mt-0.5" />
              <span className="text-sm text-red-300">{errorMsg}</span>
            </div>
            <button
              onClick={() => { setPhase('idle'); setErrorMsg('') }}
              className="text-xs text-white/35 hover:text-white/60 transition-colors font-mono underline underline-offset-2"
            >
              Dismiss
            </button>
          </div>
        )}
      </section>

      {/* Info */}
      <section className="mb-8">
        <h2 className="text-xs font-semibold uppercase tracking-widest text-white/30 mb-3">Info</h2>
        <div className="rounded-xl border border-white/5 bg-white/2 px-4">
          {[
            { label: 'sqlite_version', value: stats?.sqlite_version },
            { label: 'file_size',      value: stats ? formatBytes(stats.file_size_bytes) : undefined },
            { label: 'migrations',     value: stats?.migration_count },
          ].map(({ label, value }) => (
            <div key={label} className="flex items-center justify-between py-2.5 border-b border-white/5 last:border-0">
              <span className="text-sm text-white/40 font-mono">{label}</span>
              <span className="text-sm text-white/75 font-mono">
                {value !== undefined ? String(value) : <span className="text-white/20">—</span>}
              </span>
            </div>
          ))}
        </div>
      </section>

      {/* Tables */}
      <section className="mb-8">
        <h2 className="text-xs font-semibold uppercase tracking-widest text-white/30 mb-3">Tables</h2>
        <div className="rounded-xl border border-white/5 bg-white/2 px-4 py-4">
          {isLoading ? (
            <div className="space-y-3">
              {Array.from({ length: 8 }).map((_, i) => (
                <div key={i} className="flex items-center gap-3">
                  <div className="h-3 w-36 bg-white/5 rounded animate-pulse" />
                  <div className="flex-1 h-1.5 bg-white/5 rounded-full animate-pulse" />
                  <div className="h-3 w-8 bg-white/5 rounded animate-pulse" />
                </div>
              ))}
            </div>
          ) : (
            <div className="space-y-2.5">
              {stats?.tables.map(t => (
                <div key={t.name} className="flex items-center gap-3">
                  <span className="text-xs text-white/40 font-mono w-44 truncate shrink-0">{t.name}</span>
                  <div className="flex-1 h-1.5 rounded-full bg-white/5 overflow-hidden">
                    <div
                      className="h-full rounded-full transition-all duration-500"
                      style={{ width: `${(t.rows / maxRows) * 100}%`, background: 'rgba(99,102,241,0.5)' }}
                    />
                  </div>
                  <span className="text-xs text-white/50 font-mono w-10 text-right shrink-0">
                    {t.rows.toLocaleString()}
                  </span>
                </div>
              ))}
              {stats?.tables.length === 0 && (
                <p className="text-xs text-white/20 font-mono text-center py-4">no tables found</p>
              )}
            </div>
          )}
        </div>
      </section>
    </div>
  )
}
