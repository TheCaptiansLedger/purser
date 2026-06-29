import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import type { ReactNode } from 'react'
import { ArrowLeft, User, Film } from 'lucide-react'
import { usePerson } from '../../api/people'
import { useItems } from '../../api/items'
import { useImageVersion } from '../../hooks/useImageVersion'
import { StatusFilterChips } from '../../components/media/StatusFilterChips'
import { EditButton } from '../../components/EditButton'
import { PersonEditor } from '../../components/edit/editors/PersonEditor'
import { Badge } from '../../components/ui/Badge'
import { CountryChip } from '../../components/ui/CountryChip'
import { ItemCard } from '../../components/media/ItemCard'
import { Skeleton } from '../../components/ui/Skeleton'
import { fmtDate } from '../../components/ui/Runtime'
import type { ItemStatus } from '../../types'

const ACCENT = '#f43f5e'

function MetaRow({ label, value }: { label: string; value?: ReactNode }) {
  if (value === undefined || value === null || value === '') return null
  return (
    <div className="py-3 border-b border-white/5">
      <p className="text-xs text-white/35 mb-0.5">{label}</p>
      <p className="text-sm text-white/75">{value}</p>
    </div>
  )
}

export function PerformerDetail() {
  const { id } = useParams<{ id: string }>()
  const [editOpen, setEditOpen] = useState(false)
  const [statusFilter, setStatusFilter] = useState<ItemStatus | undefined>(undefined)
  const { data: person, isLoading } = usePerson(id!)
  const [versionedImageUrl, bumpImageVersion] = useImageVersion(person?.imageUrl)
  const { data: scenesPage } = useItems({ personId: id!, status: statusFilter, limit: 48 })
  const scenes = scenesPage?.data ?? []

  if (isLoading) return (
    <div className="flex gap-0 min-h-screen">
      <div className="w-80 shrink-0 bg-white/3 animate-pulse" />
      <div className="flex-1 px-8 py-6 space-y-4">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-4 w-64" />
        <Skeleton className="h-32 w-full" />
      </div>
    </div>
  )

  if (!person) return null

  const meta = (person.metadata ?? {}) as Record<string, string | number | boolean | null>

  return (
    <div className="flex min-h-screen">
      {/* Left panel — sticky photo + metadata */}
      <aside className="w-80 shrink-0 sticky top-0 h-screen overflow-y-auto flex flex-col border-r border-white/5">
        {/* Photo */}
        <div className="relative" style={{ aspectRatio: '2/3' }}>
          {versionedImageUrl ? (
            <img src={versionedImageUrl} alt={person.name} className="w-full h-full object-cover object-top" />
          ) : (
            <div className="w-full h-full bg-white/3 flex items-center justify-center">
              <User size={64} className="text-white/10" strokeWidth={1} />
            </div>
          )}
          <div className="absolute inset-0 bg-gradient-to-t from-[#08080e] via-transparent to-transparent" />
        </div>

        {/* Meta below photo */}
        <div className="px-5 pb-6 -mt-8 relative z-10">
          <div className="flex flex-wrap gap-1.5 mb-3">
            {person.monitored && <Badge color={ACCENT}>Monitored</Badge>}
          </div>

          {person.aliases.length > 0 && (
            <div className="mb-4">
              <p className="text-xs text-white/30 uppercase tracking-wider mb-1.5">Also known as</p>
              <div className="flex flex-wrap gap-1.5">
                {person.aliases.map(a => (
                  <span key={a} className="text-xs text-white/50 bg-white/5 px-2 py-0.5 rounded-md">{a}</span>
                ))}
              </div>
            </div>
          )}

          {/* Type-specific metadata */}
          {meta.birthdate && <MetaRow label="Born" value={fmtDate(String(meta.birthdate))} />}
          {meta.birthplace && <MetaRow label="Birthplace" value={String(meta.birthplace)} />}
          {meta.nationality && (
            <MetaRow label="Nationality" value={<CountryChip value={String(meta.nationality)} />} />
          )}
          {meta.ethnicity && <MetaRow label="Ethnicity" value={String(meta.ethnicity)} />}
          {meta.hair_color && <MetaRow label="Hair" value={String(meta.hair_color)} />}
          {meta.eye_color && <MetaRow label="Eyes" value={String(meta.eye_color)} />}
          {meta.measurements && <MetaRow label="Measurements" value={String(meta.measurements)} />}
          {meta.height && <MetaRow label="Height" value={String(meta.height)} />}
          {meta.weight && <MetaRow label="Weight" value={String(meta.weight)} />}
          {meta.cup_size && <MetaRow label="Cup Size" value={String(meta.cup_size)} />}
          {meta.breast_type && <MetaRow label="Breast Type" value={String(meta.breast_type)} />}
          {meta.tattoos && <MetaRow label="Tattoos" value={String(meta.tattoos)} />}
          {meta.piercings && <MetaRow label="Piercings" value={String(meta.piercings)} />}
          {meta.career_start && <MetaRow label="Career start" value={String(meta.career_start)} />}

          {/* External IDs */}
          {person.externalIds.length > 0 && (
            <div className="mt-4 pt-3 border-t border-white/5">
              <p className="text-xs text-white/30 uppercase tracking-wider mb-2">External IDs</p>
              <div className="flex flex-col gap-1">
                {person.externalIds.map(e => (
                  <span key={e.source} className="text-xs text-white/40">
                    <span className="text-white/25">{e.source}:</span> {e.value}
                  </span>
                ))}
              </div>
            </div>
          )}
        </div>
      </aside>

      {/* Right panel — bio + scenes */}
      <div className="flex-1 min-w-0 overflow-y-auto">
        <div className="px-8 pt-6 flex items-center justify-between">
          <Link to="/afterdark" className="inline-flex items-center gap-1.5 text-sm text-white/40 hover:text-white/70 transition-colors">
            <ArrowLeft size={14} /> AfterDark
          </Link>
          <EditButton onClick={() => setEditOpen(true)} />
        </div>

        <div className="px-8 py-6 space-y-10">
          {/* Header */}
          <div>
            <h1 className="text-3xl font-bold text-white mb-1">{person.name}</h1>
            {scenes.length > 0 && (
              <p className="text-sm text-white/40">{scenesPage?.total} scene{scenesPage?.total !== 1 ? 's' : ''}</p>
            )}
          </div>

          {/* Bio */}
          {person.overview && (
            <section>
              <h2 className="text-xs font-semibold text-white/35 uppercase tracking-widest mb-3">Biography</h2>
              <p className="text-sm text-white/60 leading-relaxed max-w-3xl">{person.overview}</p>
            </section>
          )}

          {/* Scenes grid */}
          <section>
            <h2 className="text-xs font-semibold text-white/35 uppercase tracking-widest mb-3 flex items-center gap-2">
              <Film size={13} style={{ color: ACCENT }} />
              Scenes
            </h2>
            <div className="mb-4">
              <StatusFilterChips value={statusFilter} onChange={setStatusFilter} accent={ACCENT} />
            </div>
            {scenes.length === 0 ? (
              <p className="text-white/30 text-sm">{statusFilter ? 'No scenes match this filter.' : 'No scenes yet.'}</p>
            ) : (
              <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4">
                {scenes.map(scene => (
                  <ItemCard key={scene.id} item={scene} href={`/afterdark/scenes/${scene.id}`} aspect="16/9" accent={ACCENT} showPeople alwaysShowStatus={statusFilter !== undefined} />
                ))}
              </div>
            )}
          </section>
        </div>
      </div>

      {editOpen && (
        <PersonEditor
          person={person}
          onClose={() => setEditOpen(false)}
          onImageSet={bumpImageVersion}
        />
      )}
    </div>
  )
}
