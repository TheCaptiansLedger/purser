import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { Calendar, Clock, ImageIcon, User } from 'lucide-react'
import { useItem } from '../../api/items'
import { useLibraryEntry } from '../../api/library'
import { EditButton } from '../../components/EditButton'
import { ItemEditor } from '../../components/edit/editors/ItemEditor'
import { Hero } from '../../components/layout/Hero'
import { Badge } from '../../components/ui/Badge'
import { Lightbox } from '../../components/ui/Lightbox'
import { PersonCard } from '../../components/media/PersonCard'
import { fmtRuntime, fmtDate, fmtBytes } from '../../components/ui/Runtime'
import { Skeleton } from '../../components/ui/Skeleton'
import { ExpandableText } from '../../components/ui/ExpandableText'

const ACCENT = '#f43f5e'

export function SceneDetail() {
  const { id } = useParams<{ id: string }>()
  const [editOpen, setEditOpen] = useState(false)
  const [lightboxOpen, setLightboxOpen] = useState(false)

  const { data: item, isLoading } = useItem(id!)
  const { data: entry } = useLibraryEntry(item?.libraryEntryId ?? '')

  if (isLoading) return <div className="px-8 py-10"><Skeleton className="h-64 w-full" /></div>
  if (!item) return null

  const performers = item.people.filter(p => p.role === 'performer' || p.role === 'actress' || p.role === 'actor')
  const directors  = item.people.filter(p => p.role === 'director')

  return (
    <div>
      <div className="px-8 pt-6 flex items-center justify-between">
        <nav className="flex items-center gap-1.5 text-sm text-white/40">
          <Link to="/afterdark/studios" className="hover:text-white/70 transition-colors">Studios</Link>
          {entry && (
            <>
              <span className="text-white/20">›</span>
              <Link to={`/afterdark/studios/${entry.id}`} className="hover:text-white/70 transition-colors">{entry.name}</Link>
            </>
          )}
          <span className="text-white/20">›</span>
          <span className="text-white/60 truncate max-w-xs">{item.title}</span>
        </nav>
        <EditButton onClick={() => setEditOpen(true)} className="shrink-0" />
      </div>

      {/* Hero — big thumbnail left, title + tags right */}
      <Hero backdropUrl={item.coverUrl} accent={ACCENT}>
        <div className="flex flex-col lg:flex-row gap-8 items-start lg:items-center">

          {/* Big 16:9 thumbnail */}
          <div
            className="shrink-0 w-full lg:w-[45%] lg:max-w-2xl rounded-2xl overflow-hidden border border-white/10 shadow-2xl"
            style={{ aspectRatio: '16/9' }}
          >
            {item.coverUrl ? (
              <button
                className="block w-full h-full cursor-zoom-in"
                onClick={() => setLightboxOpen(true)}
                aria-label={`View ${item.title} thumbnail`}
              >
                <img src={item.coverUrl} alt={item.title} className="w-full h-full object-cover" />
              </button>
            ) : (
              <div className="w-full h-full bg-white/5 flex items-center justify-center">
                <ImageIcon size={48} className="text-white/15" strokeWidth={1} />
              </div>
            )}
          </div>

          {/* Title, metadata, tags */}
          <div className="flex-1 min-w-0">
            {entry && (
              <Link
                to={`/afterdark/studios/${entry.id}`}
                className="text-xs font-medium uppercase tracking-widest hover:opacity-80 transition-opacity mb-3 block"
                style={{ color: ACCENT }}
              >
                {entry.name}
              </Link>
            )}
            <h1 className="text-4xl xl:text-5xl font-bold text-white leading-tight mb-4">
              {item.title}
            </h1>
            <div className="flex flex-wrap items-center gap-3 mb-5">
              {item.date && (
                <span className="flex items-center gap-1.5 text-sm text-white/50">
                  <Calendar size={14} />{fmtDate(item.date)}
                </span>
              )}
              {item.runtimeSeconds > 0 && (
                <span className="flex items-center gap-1.5 text-sm text-white/50">
                  <Clock size={14} />{fmtRuntime(item.runtimeSeconds)}
                </span>
              )}
              {item.mediaFile?.quality && <Badge>{item.mediaFile.quality}</Badge>}
              {item.sequence && <Badge color={ACCENT}>{item.sequence}</Badge>}
            </div>
            {performers.length > 0 && (
              <div className="flex flex-wrap gap-1.5 mb-4">
                {performers.slice(0, 6).map(({ person, personId }) => (
                  <Link
                    key={personId}
                    to={`/afterdark/performers/${personId}`}
                    className="flex items-center gap-1.5 text-xs bg-white/8 hover:bg-white/14 px-2.5 py-1 rounded-full text-white/70 hover:text-white transition-all"
                  >
                    {person?.imageUrl ? (
                      <img src={person.imageUrl} alt={person.name} className="w-4 h-4 rounded-full object-cover" />
                    ) : (
                      <User size={12} />
                    )}
                    {person?.name ?? personId}
                  </Link>
                ))}
              </div>
            )}
            {item.tags.length > 0 && (
              <div className="flex flex-wrap gap-1.5">
                {item.tags.map(t => (
                  <Link key={t.id} to={`/tags/${encodeURIComponent(t.key)}/${encodeURIComponent(t.value)}`}>
                    <Badge>{t.value}</Badge>
                  </Link>
                ))}
              </div>
            )}
          </div>
        </div>
      </Hero>

      {/* Body */}
      <div className="px-8 py-8 space-y-10">

        {/* Two-column: performers left, description right */}
        {(performers.length > 0 || directors.length > 0 || item.overview) && (
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-10">

            {/* Left — performers grid + director */}
            <div className="space-y-8">
              {performers.length > 0 && (
                <section>
                  <h2 className="text-sm font-semibold text-white/40 uppercase tracking-widest mb-4">Performers</h2>
                  <div className="grid grid-cols-2 sm:grid-cols-4 lg:grid-cols-2 xl:grid-cols-3 gap-3">
                    {performers.map(({ person, personId }) => person ? (
                      <PersonCard
                        key={personId}
                        person={person}
                        href={`/afterdark/performers/${personId}`}
                        accent={ACCENT}
                      />
                    ) : null)}
                  </div>
                </section>
              )}

              {directors.length > 0 && (
                <section>
                  <h2 className="text-sm font-semibold text-white/40 uppercase tracking-widest mb-3">Director</h2>
                  <div className="flex flex-wrap gap-2">
                    {directors.map(({ person, personId }) => (
                      <span key={personId} className="text-sm text-white/60 bg-white/5 px-3 py-1.5 rounded-lg">
                        {person?.name ?? personId}
                      </span>
                    ))}
                  </div>
                </section>
              )}
            </div>

            {/* Right — description, bigger text, clamped */}
            <div className="lg:col-span-2">
              {item.overview && (
                <section>
                  <h2 className="text-sm font-semibold text-white/40 uppercase tracking-widest mb-3">Description</h2>
                  <ExpandableText
                    text={item.overview}
                    maxLines={8}
                    textClassName="text-lg text-white/70 leading-relaxed"
                  />
                </section>
              )}
            </div>
          </div>
        )}

        {/* File info — full width */}
        {item.mediaFile && (
          <section>
            <h2 className="text-sm font-semibold text-white/40 uppercase tracking-widest mb-3">File</h2>
            <div className="grid grid-cols-2 md:grid-cols-3 xl:grid-cols-6 gap-3">
              {[
                { label: 'Size',      value: fmtBytes(item.mediaFile.size)     },
                { label: 'Quality',   value: item.mediaFile.quality             },
                { label: 'Resolution',value: item.mediaFile.resolution          },
                { label: 'Codec',     value: item.mediaFile.codec               },
                { label: 'Container', value: item.mediaFile.container           },
                { label: 'Path',      value: item.mediaFile.path                },
              ].filter(r => r.value).map(({ label, value }) => (
                <div key={label} className="bg-white/3 rounded-lg p-3">
                  <p className="text-xs text-white/35 mb-0.5">{label}</p>
                  <p className="text-sm text-white/80 truncate">{value}</p>
                </div>
              ))}
            </div>
          </section>
        )}
      </div>

      {editOpen && <ItemEditor item={item} onClose={() => setEditOpen(false)} hideTagKeys={['adult']} />}

      {lightboxOpen && item.coverUrl && (
        <Lightbox src={item.coverUrl} alt={item.title} onClose={() => setLightboxOpen(false)} />
      )}
    </div>
  )
}
