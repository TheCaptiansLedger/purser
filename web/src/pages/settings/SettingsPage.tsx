import { Film, Tv2, Music2, BookOpen, Sparkles, Video, FolderOpen } from 'lucide-react'
import { useAppConfig } from '../../api/config'
import type { EnabledModules } from '../../context/ModulesContext'

const MODULE_META: Array<{
  key: keyof EnabledModules
  label: string
  description: string
  icon: React.ElementType
  accent: string
}> = [
  { key: 'movies',    label: 'Movies',    description: 'Feature films and short films',             icon: Film,     accent: '#3b82f6' },
  { key: 'tv',        label: 'TV Shows',  description: 'Series, seasons, and episodes',             icon: Tv2,      accent: '#8b5cf6' },
  { key: 'music',     label: 'Music',     description: 'Artists, albums, and tracks',               icon: Music2,   accent: '#10b981' },
  { key: 'books',     label: 'Books',     description: 'Books, series, and authors',                icon: BookOpen, accent: '#f59e0b' },
  { key: 'afterdark', label: 'AfterDark', description: 'Adult content — networks, studios, scenes', icon: Sparkles, accent: '#f43f5e' },
  { key: 'jav',       label: 'JAV',       description: 'Japanese adult video — studios and titles', icon: Video,    accent: '#ec4899' },
]

function Row({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="flex items-center justify-between py-2.5 border-b border-white/5 last:border-0">
      <span className="text-sm text-white/40 font-mono">{label}</span>
      <span className="text-sm text-white/75 font-mono">{String(value)}</span>
    </div>
  )
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="mb-8">
      <h2 className="text-xs font-semibold uppercase tracking-widest text-white/30 mb-3">{title}</h2>
      <div className="rounded-xl border border-white/5 bg-white/2 px-4">
        {children}
      </div>
    </section>
  )
}

function RootsList({ roots }: { roots: string[] }) {
  if (roots.length === 0) {
    return (
      <div className="py-2.5 border-b border-white/5 last:border-0">
        <span className="text-xs text-white/20 font-mono italic">no roots configured</span>
      </div>
    )
  }
  return (
    <>
      {roots.map((root) => (
        <div key={root} className="flex items-center gap-2 py-2 border-b border-white/5 last:border-0">
          <FolderOpen size={13} className="text-white/25 shrink-0" />
          <span className="text-sm text-white/60 font-mono truncate">{root}</span>
        </div>
      ))}
    </>
  )
}

export function SettingsPage() {
  const { data: cfg, isLoading } = useAppConfig()

  if (isLoading || !cfg) {
    return (
      <div className="px-8 py-10 max-w-2xl">
        <div className="h-8 w-48 bg-white/5 rounded animate-pulse mb-10" />
        <div className="h-40 bg-white/3 rounded-xl animate-pulse" />
      </div>
    )
  }

  return (
    <div className="px-8 py-10 max-w-2xl">
      <div className="mb-10">
        <h1 className="text-2xl font-semibold text-white">Settings</h1>
        <p className="text-white/40 text-sm mt-1">
          Edit <code className="text-white/60 font-mono text-xs">purser.yaml</code> or set{' '}
          <code className="text-white/60 font-mono text-xs">PURSER_*</code> env vars to change
        </p>
      </div>

      <Section title="Server">
        <Row label="port" value={cfg.server.port} />
      </Section>

      <Section title="Database">
        <Row label="driver" value={cfg.database.driver} />
        <Row label="dsn"    value={cfg.database.dsn} />
      </Section>

      <Section title="Media">
        <Row label="path" value={cfg.media.path} />
      </Section>

      <Section title="Log">
        <Row label="level"  value={cfg.log.level} />
        <Row label="format" value={cfg.log.format} />
      </Section>

      <section className="mb-8">
        <h2 className="text-xs font-semibold uppercase tracking-widest text-white/30 mb-3">Modules</h2>
        <div className="flex flex-col gap-2">
          {MODULE_META.map(({ key, label, description, icon: Icon, accent }) => {
            const mod = cfg.modules[key]
            const enabled = mod.enabled
            return (
              <div
                key={key}
                className="rounded-xl border border-white/5 bg-white/2 overflow-hidden"
              >
                <div className="flex items-center gap-4 px-4 py-3.5">
                  <div
                    className="w-9 h-9 rounded-lg flex items-center justify-center shrink-0"
                    style={{ background: enabled ? accent + '22' : 'rgba(255,255,255,0.04)' }}
                  >
                    <Icon size={17} strokeWidth={2} style={{ color: enabled ? accent : 'rgba(255,255,255,0.2)' }} />
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="text-sm font-medium text-white/80">{label}</div>
                    <div className="text-xs text-white/35 mt-0.5 truncate">{description}</div>
                  </div>
                  <span className={[
                    'text-xs font-medium px-2 py-0.5 rounded-full shrink-0',
                    enabled ? 'bg-emerald-500/15 text-emerald-400' : 'bg-white/5 text-white/25',
                  ].join(' ')}>
                    {enabled ? 'enabled' : 'disabled'}
                  </span>
                </div>
                {enabled && (
                  <div className="border-t border-white/5 px-4 pb-1">
                    <div className="text-[10px] font-semibold uppercase tracking-widest text-white/20 pt-2.5 pb-1">
                      Scan Roots
                    </div>
                    <RootsList roots={mod.roots} />
                  </div>
                )}
              </div>
            )
          })}
        </div>
      </section>
    </div>
  )
}
