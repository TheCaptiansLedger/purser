import { useMemo, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { ArrowLeft, ImageIcon, ChevronLeft, ChevronRight, Disc3, Users, ArrowUpNarrowWide, ArrowDownNarrowWide, RefreshCw } from 'lucide-react'
import { useLibraryEntry } from '../../api/library'
import { useGroups, sortGroupsByYear } from '../../api/groups'
import { useImageVersion } from '../../hooks/useImageVersion'
import type { YearSortDir } from '../../api/groups'
import { useActiveJobForEntry } from '../../api/jobs'
import { refreshArtist } from '../../api/commands'
import { AlbumCard } from '../../components/AlbumCard'
import { EditButton } from '../../components/EditButton'
import { LibraryEntryEditor } from '../../components/edit/editors/LibraryEntryEditor'
import { Hero } from '../../components/layout/Hero'
import { PersonCard } from '../../components/media/PersonCard'
import { Lightbox } from '../../components/ui/Lightbox'
import { Skeleton } from '../../components/ui/Skeleton'
import type { Group } from '../../types'

const ACCENT = '#10b981'
const PAGE_SIZE = 6

type ArtistTab = 'discography' | 'members'

type DiscographySection = { label: string; token: string }

const SECTIONS: DiscographySection[] = [
  { label: 'Albums',        token: 'studio'      },
  { label: 'Live',          token: 'live'         },
  { label: 'EPs & Singles', token: 'ep_single'   },
  { label: 'Compilations',  token: 'compilation' },
  { label: 'Other',         token: 'other'        },
]

function albumSectionToken(album: Group): string {
  const primary    = (album.metadata?.primary_type   as string   | undefined) ?? ''
  const secondary  = (album.metadata?.secondary_types as string[] | undefined) ?? []
  if (primary === 'EP' || primary === 'Single') return 'ep_single'
  if (primary !== 'Album') return 'other'
  if (secondary.includes('Live'))        return 'live'
  if (secondary.includes('Compilation')) return 'compilation'
  return 'studio'
}

// ── Section with arrow pagination ────────────────────────────────────────────

function DiscographySection({
  section,
  albums,
  artistId,
  sortDir,
}: {
  section: DiscographySection
  albums: Group[]
  artistId: string
  sortDir: YearSortDir
}) {
  const [page, setPage] = useState(0)
  if (albums.length === 0) return null

  const sorted = sortGroupsByYear(albums, sortDir)
  const pages = Math.ceil(sorted.length / PAGE_SIZE)
  const slice = sorted.slice(page * PAGE_SIZE, (page + 1) * PAGE_SIZE)

  return (
    <div>
      <div className="flex items-center justify-between mb-3">
        <h3 className="text-xs font-semibold text-white/30 uppercase tracking-widest">
          {section.label}
          <span className="ml-2 font-normal normal-case tracking-normal text-white/20">{albums.length}</span>
        </h3>
        {pages > 1 && (
          <div className="flex items-center gap-1">
            <button
              onClick={() => setPage(p => p - 1)}
              disabled={page === 0}
              className="flex items-center justify-center w-7 h-7 rounded-lg text-white/35 hover:text-white/70 hover:bg-white/5 disabled:opacity-20 disabled:cursor-not-allowed transition-all"
            >
              <ChevronLeft size={14} />
            </button>
            <span className="text-xs text-white/20 w-10 text-center tabular-nums">
              {page + 1} / {pages}
            </span>
            <button
              onClick={() => setPage(p => p + 1)}
              disabled={page >= pages - 1}
              className="flex items-center justify-center w-7 h-7 rounded-lg text-white/35 hover:text-white/70 hover:bg-white/5 disabled:opacity-20 disabled:cursor-not-allowed transition-all"
            >
              <ChevronRight size={14} />
            </button>
          </div>
        )}
      </div>

      <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-4">
        {slice.map(album => (
          <AlbumCard
            key={album.id}
            album={album}
            href={`/music/${artistId}/albums/${album.id}`}
            showMonitorBadge
            accent={ACCENT}
          />
        ))}
      </div>
    </div>
  )
}

// ── Page ──────────────────────────────────────────────────────────────────────

export function ArtistDetail() {
  const { id } = useParams<{ id: string }>()
  const [tab, setTab] = useState<ArtistTab>('discography')
  const [sortDir, setSortDir] = useState<YearSortDir>('desc')
  const [submitting, setSubmitting] = useState(false)
  const [editOpen, setEditOpen] = useState(false)
  const [lightboxOpen, setLightboxOpen] = useState(false)

  const activeJob   = useActiveJobForEntry(id!, 'RefreshArtist')
  const isImporting = activeJob !== null

  const { data: entry, isLoading } = useLibraryEntry(id!)
  const [versionedImageUrl, bumpImageVersion] = useImageVersion(entry?.imageUrl)
  const { data: albumsPage } = useGroups(id!, isImporting ? 2000 : undefined)
  const albums = albumsPage?.data ?? []

  const handleRefresh = async () => {
    if (submitting || isImporting) return
    setSubmitting(true)
    try {
      await refreshArtist(id!)
    } finally {
      setSubmitting(false)
    }
  }

  const bySection = useMemo(
    () => Object.fromEntries(
      SECTIONS.map(s => [s.token, albums.filter(a => albumSectionToken(a) === s.token)])
    ),
    [albums]
  )

  if (isLoading) return <div className="px-8 py-10"><Skeleton className="h-64 w-full" /></div>
  if (!entry) return null

  const TABS: { id: ArtistTab; label: string; icon: typeof Disc3 }[] = [
    { id: 'discography', label: 'Discography', icon: Disc3  },
    { id: 'members',     label: 'Members',     icon: Users  },
  ]

  const refreshLabel = isImporting
    ? activeJob.message ?? `${activeJob.current}/${activeJob.total} albums`
    : 'Refresh'

  return (
    <div>
      <div className="px-8 pt-6 flex items-center justify-between">
        <Link to="/music" className="inline-flex items-center gap-1.5 text-sm text-white/40 hover:text-white/70 transition-colors">
          <ArrowLeft size={14} /> Music
        </Link>

        <div className="flex items-center gap-2">
          <EditButton onClick={() => setEditOpen(true)} />
          <button
            onClick={handleRefresh}
            disabled={submitting || isImporting}
            className="inline-flex items-center gap-1.5 text-xs font-medium px-3 py-1.5 rounded-lg border border-white/10 text-white/50 hover:text-white/80 hover:border-white/20 transition-colors disabled:opacity-50 disabled:cursor-default"
          >
            <RefreshCw size={12} className={isImporting || submitting ? 'animate-spin' : ''} />
            {submitting ? 'Starting…' : refreshLabel}
          </button>
        </div>
      </div>

      <Hero backdropUrl={entry.imageUrl} accent={ACCENT}>
        <div className="flex gap-6 items-end">
          <div className="shrink-0 w-36 h-36 rounded-full overflow-hidden border-2 shadow-2xl" style={{ borderColor: ACCENT + '44' }}>
            {entry.imageUrl ? (
              <button
                className="block w-full h-full cursor-zoom-in"
                onClick={() => setLightboxOpen(true)}
                aria-label={`View ${entry.name} avatar`}
              >
                <img src={versionedImageUrl} alt={entry.name} className="w-full h-full object-cover" />
              </button>
            ) : (
              <div className="w-full h-full bg-white/5 flex items-center justify-center">
                <ImageIcon size={32} className="text-white/15" strokeWidth={1} />
              </div>
            )}
          </div>
          <div>
            <p className="text-xs font-medium uppercase tracking-widest mb-1" style={{ color: ACCENT }}>Artist</p>
            <h1 className="text-4xl font-bold text-white mb-2">{entry.name}</h1>
            <div className="flex items-center gap-3 text-sm text-white/35">
              {albums.length > 0 && <span>{albums.length} release{albums.length !== 1 ? 's' : ''}</span>}
              {entry.people.length > 0 && <span>{entry.people.length} member{entry.people.length !== 1 ? 's' : ''}</span>}
              {isImporting && <span className="italic">importing…</span>}
            </div>
          </div>
        </div>
      </Hero>

      <div className="px-8 py-6">
        {entry.overview && (
          <p className="text-sm text-white/60 leading-relaxed max-w-3xl mb-6">{entry.overview}</p>
        )}

        <div className="flex items-center justify-between mb-8">
          <div className="flex gap-1">
            {TABS.map(({ id: tid, label, icon: Icon }) => (
              <button
                key={tid}
                onClick={() => setTab(tid)}
                className={[
                  'flex items-center gap-1.5 px-3 h-8 rounded-lg text-xs font-medium transition-all duration-150',
                  tab === tid
                    ? 'text-white'
                    : 'text-white/40 hover:text-white/65 hover:bg-white/5',
                ].join(' ')}
                style={tab === tid ? { background: ACCENT + '28', color: ACCENT } : {}}
              >
                <Icon size={13} />
                {label}
              </button>
            ))}
          </div>
          {tab === 'discography' && (
            <button
              type="button"
              onClick={() => setSortDir(d => d === 'asc' ? 'desc' : 'asc')}
              title={sortDir === 'asc' ? 'Oldest first — click for newest first' : 'Newest first — click for oldest first'}
              className="inline-flex items-center gap-1.5 text-xs px-2.5 py-1.5 rounded-lg border transition-colors"
              style={{ borderColor: 'rgba(255,255,255,0.1)', color: 'rgba(255,255,255,0.4)' }}
            >
              {sortDir === 'asc' ? <ArrowUpNarrowWide size={13} /> : <ArrowDownNarrowWide size={13} />}
              {sortDir === 'asc' ? 'Oldest first' : 'Newest first'}
            </button>
          )}
        </div>

        {tab === 'discography' && (
          albums.length === 0 ? (
            <p className="text-white/30 text-sm">No albums added yet.</p>
          ) : (
            <div className="space-y-8">
              {SECTIONS.map(section => (
                <DiscographySection
                  key={section.token}
                  section={section}
                  albums={bySection[section.token] ?? []}
                  artistId={id!}
                  sortDir={sortDir}
                />
              ))}
            </div>
          )
        )}

        {tab === 'members' && (
          entry.people.length === 0 ? (
            <p className="text-white/30 text-sm">No members listed.</p>
          ) : (
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-4">
              {entry.people.map(ep => ep.person && (
                <PersonCard
                  key={ep.personId}
                  person={ep.person}
                  href={`/people/${ep.personId}`}
                  accent={ACCENT}
                />
              ))}
            </div>
          )
        )}
      </div>

      {editOpen && (
        <LibraryEntryEditor
          entry={entry}
          onClose={() => setEditOpen(false)}
          onImageSet={bumpImageVersion}
        />
      )}

      {lightboxOpen && entry.imageUrl && (
        <Lightbox src={entry.imageUrl} alt={entry.name} onClose={() => setLightboxOpen(false)} />
      )}
    </div>
  )
}
