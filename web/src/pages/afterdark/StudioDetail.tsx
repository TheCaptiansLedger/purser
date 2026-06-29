import { useMemo, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { ArrowLeft, Film, Users, ImageIcon, RefreshCw } from 'lucide-react'
import { useLibraryEntry } from '../../api/library'
import { useItems } from '../../api/items'
import { useImageVersion } from '../../hooks/useImageVersion'
import type { SortField, SortDir } from '../../api/items'
import { useActiveJobForEntry } from '../../api/jobs'
import { refreshStudio } from '../../api/commands'
import { useStatusOverlay } from '../../hooks/useStatusOverlay'
import { EditButton } from '../../components/EditButton'
import { LibraryEntryEditor } from '../../components/edit/editors/LibraryEntryEditor'
import { EntryHero } from '../../components/layout/EntryHero'
import { ItemCard } from '../../components/media/ItemCard'
import { PersonCard } from '../../components/media/PersonCard'
import { Badge } from '../../components/ui/Badge'
import { Pagination } from '../../components/ui/Pagination'
import { Skeleton } from '../../components/ui/Skeleton'
import type { PersonRef } from '../../types'

const ACCENT = '#f43f5e'
const LIMIT = 48

export function StudioDetail() {
  const { id } = useParams<{ id: string }>()
  const { data: entry, isLoading } = useLibraryEntry(id!)

  const activeJob = useActiveJobForEntry(id!, 'RefreshStudio')
  const isRefreshing = activeJob !== null

  const [sceneOffset, setSceneOffset] = useState(0)
  const [sort, setSort] = useState<SortField>('date')
  const [sortDir, setSortDir] = useState<SortDir>('desc')
  const [alwaysShowStatus, toggleStatus] = useStatusOverlay('afterdark')

  const changeSort = (newSort: SortField) => {
    if (newSort === sort) {
      setSortDir(d => d === 'desc' ? 'asc' : 'desc')
    } else {
      setSort(newSort)
      setSortDir(newSort === 'date' ? 'desc' : 'asc')
    }
    setSceneOffset(0)
  }

  const { data: scenesPage } = useItems(
    { libraryEntryId: id!, sort, sortDir, limit: LIMIT, offset: sceneOffset },
    isRefreshing ? 2000 : undefined,
  )
  const scenes = scenesPage?.data ?? []

  const [editOpen, setEditOpen] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [versionedImageUrl, bumpImageVersion] = useImageVersion(entry?.imageUrl)

  const handleRefresh = async () => {
    if (submitting || isRefreshing) return
    setSubmitting(true)
    try {
      await refreshStudio(id!)
    } finally {
      setSubmitting(false)
    }
  }

  const performers = useMemo(() => {
    const map = new Map<string, PersonRef>()
    for (const scene of scenes) {
      for (const ip of scene.people) {
        if (!map.has(ip.personId) && ip.person) {
          map.set(ip.personId, ip.person)
        }
      }
    }
    return Array.from(map.values()).sort((a, b) => a.sortName.localeCompare(b.sortName))
  }, [scenes])

  if (isLoading) return (
    <div className="px-8 py-10 space-y-4">
      <Skeleton className="h-64 w-full" />
      <Skeleton className="h-8 w-48" />
    </div>
  )
  if (!entry) return null

  const refreshLabel = isRefreshing
    ? activeJob.message ?? `${activeJob.current}/${activeJob.total} scenes`
    : 'Refresh'

  return (
    <div>
      <div className="px-8 pt-6 flex items-center justify-between">
        <Link to="/afterdark/studios" className="inline-flex items-center gap-1.5 text-sm text-white/40 hover:text-white/70 transition-colors">
          <ArrowLeft size={14} /> Studios
        </Link>

        <div className="flex items-center gap-2">
          <EditButton onClick={() => setEditOpen(true)} />
          <button
            onClick={handleRefresh}
            disabled={submitting || isRefreshing}
            className="inline-flex items-center gap-1.5 text-xs font-medium px-3 py-1.5 rounded-lg border border-white/10 text-white/50 hover:text-white/80 hover:border-white/20 transition-colors disabled:opacity-50 disabled:cursor-default"
          >
            <RefreshCw
              size={12}
              className={isRefreshing || submitting ? 'animate-spin' : ''}
            />
            {submitting ? 'Starting…' : refreshLabel}
          </button>
        </div>
      </div>

      <EntryHero entry={entry} backdropFallbackUrl={versionedImageUrl} accent={ACCENT}>
        <div className="flex gap-6 items-end">
          <div className="shrink-0 w-40 rounded-xl overflow-hidden border border-white/10 shadow-2xl" style={{ aspectRatio: '16/9' }}>
            {versionedImageUrl ? (
              <img src={versionedImageUrl} alt={entry.name} className="w-full h-full object-contain p-2" />
            ) : (
              <div className="w-full h-full bg-white/5 flex items-center justify-center">
                <ImageIcon size={32} className="text-white/15" strokeWidth={1} />
              </div>
            )}
          </div>

          <div className="flex-1 min-w-0">
            <p className="text-xs font-medium uppercase tracking-widest mb-1" style={{ color: ACCENT }}>
              Studio
            </p>
            <h1 className="text-3xl font-bold text-white mb-2 leading-tight">{entry.name}</h1>
            <div className="flex flex-wrap items-center gap-2">
              {entry.status && entry.status !== 'active' && <Badge color="#ef4444">{entry.status}</Badge>}
              {scenes.length > 0 && (
                <span className="text-sm text-white/40">
                  {scenesPage?.total ?? scenes.length} scene{(scenesPage?.total ?? scenes.length) !== 1 ? 's' : ''}
                </span>
              )}
              {performers.length > 0 && (
                <span className="text-sm text-white/40">· {performers.length} performer{performers.length !== 1 ? 's' : ''}</span>
              )}
              {isRefreshing && (
                <span className="text-xs text-white/30 italic">importing…</span>
              )}
            </div>
          </div>
        </div>
      </EntryHero>

      <div className="px-8 py-8 space-y-12">
        {entry.overview && (
          <section>
            <h2 className="text-xs font-semibold text-white/35 uppercase tracking-widest mb-3">About</h2>
            <p className="text-sm text-white/60 leading-relaxed max-w-3xl">{entry.overview}</p>
          </section>
        )}

        {entry.tags.length > 0 && (
          <section>
            <h2 className="text-xs font-semibold text-white/35 uppercase tracking-widest mb-3">Tags</h2>
            <div className="flex flex-wrap gap-2">
              {entry.tags.map(t => (
                <Link key={t.id} to={`/tags/${encodeURIComponent(t.key)}/${encodeURIComponent(t.value)}`}>
                  <Badge>{t.value}</Badge>
                </Link>
              ))}
            </div>
          </section>
        )}

        {(scenes.length > 0 || sceneOffset > 0) && (
          <section>
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-xs font-semibold text-white/35 uppercase tracking-widest flex items-center gap-2">
                <Film size={13} style={{ color: ACCENT }} />
                Scenes
              </h2>
              <div className="flex items-center gap-2">
                <button
                  onClick={toggleStatus}
                  className={[
                    'text-xs px-2.5 py-1 rounded-lg border transition-colors',
                    alwaysShowStatus
                      ? 'border-transparent text-white'
                      : 'border-white/10 text-white/40 hover:text-white/70',
                  ].join(' ')}
                  style={alwaysShowStatus ? { background: ACCENT + '22', color: ACCENT, borderColor: ACCENT + '44' } : {}}
                >
                  Status
                </button>
                {([['date', 'Date'], ['title', 'A–Z']] as [SortField, string][]).map(([key, label]) => (
                  <button
                    key={key}
                    onClick={() => changeSort(key)}
                    className={[
                      'text-xs px-2.5 py-1 rounded-lg border transition-colors',
                      sort === key
                        ? 'border-transparent text-white'
                        : 'border-white/10 text-white/40 hover:text-white/70 hover:border-white/20',
                    ].join(' ')}
                    style={sort === key ? { background: ACCENT + '33', color: ACCENT, borderColor: ACCENT + '55' } : {}}
                  >
                    {label}{sort === key ? (sortDir === 'asc' ? ' ↑' : ' ↓') : ''}
                  </button>
                ))}
              </div>
            </div>
            <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4">
              {scenes.map(scene => (
                <ItemCard key={scene.id} item={scene} href={`/afterdark/scenes/${scene.id}`} aspect="16/9" accent={ACCENT} showPeople alwaysShowStatus={alwaysShowStatus} />
              ))}
            </div>
            <Pagination total={scenesPage?.total ?? 0} limit={LIMIT} offset={sceneOffset} onChange={setSceneOffset} accent={ACCENT} />
          </section>
        )}

        {performers.length > 0 && (
          <section>
            <h2 className="text-xs font-semibold text-white/35 uppercase tracking-widest mb-4 flex items-center gap-2">
              <Users size={13} style={{ color: ACCENT }} />
              Performers
            </h2>
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 xl:grid-cols-8 gap-3">
              {performers.map(p => (
                <PersonCard key={p.id} person={p} href={`/afterdark/performers/${p.id}`} accent={ACCENT} />
              ))}
            </div>
          </section>
        )}
      </div>

      {editOpen && (
        <LibraryEntryEditor
          entry={entry}
          onClose={() => setEditOpen(false)}
          onImageSet={bumpImageVersion}
        />
      )}
    </div>
  )
}
