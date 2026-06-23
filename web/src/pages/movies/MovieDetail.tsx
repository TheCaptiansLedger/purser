import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { ArrowLeft, Calendar, Clock, ImageIcon, Edit2 } from 'lucide-react'
import { useQueryClient } from '@tanstack/react-query'
import { useLibraryEntry, updateLibraryEntry, useAddEntryTag, useRemoveEntryTag } from '../../api/library'
import { useItems } from '../../api/items'
import { useEditForm } from '../../hooks/useEditForm'
import { EditDrawer } from '../../components/edit/EditDrawer'
import { ImageSelector } from '../../components/edit/ImageSelector'
import { FormField } from '../../components/edit/FormField'
import { TextInput } from '../../components/edit/fields/TextInput'
import { Textarea } from '../../components/edit/fields/Textarea'
import { TagPicker } from '../../components/edit/fields/TagPicker'
import { RelationshipPanel } from '../../components/edit/RelationshipPanel'
import { Hero } from '../../components/layout/Hero'
import { Badge } from '../../components/ui/Badge'
import { PersonCard } from '../../components/media/PersonCard'
import { fmtRuntime, fmtBytes } from '../../components/ui/Runtime'
import { Skeleton } from '../../components/ui/Skeleton'
import { filterTagsForModule } from '../../utils/filterTagsForModule'
import type { Item, LibraryEntry } from '../../types'

const ACCENT = '#3b82f6'

type MovieEntryFormValues = { name: string; overview: string }

function MovieEditDrawer({ entry, item, onClose, onImageSet }: { entry: LibraryEntry; item?: Item; onClose: () => void; onImageSet: () => void }) {
  const queryClient = useQueryClient()
  const addTag = useAddEntryTag(entry.id)
  const removeTag = useRemoveEntryTag(entry.id)

  const form = useEditForm<MovieEntryFormValues>({
    initial: { name: entry.name, overview: entry.overview ?? '' },
    lockedFields: entry.lockedFields,
    onSubmit: async (values, lockedFields) => {
      const updated = await updateLibraryEntry(entry.id, { ...values, lockedFields })
      queryClient.setQueryData(['library-entries', entry.id], updated)
    },
    onSuccess: onClose,
  })

  const currentEntry = queryClient.getQueryData<LibraryEntry>(['library-entries', entry.id]) ?? entry

  return (
    <EditDrawer title={entry.name} onClose={onClose} onSave={form.submit} saving={form.submitting}>
      <div className="space-y-8">
        <div className="grid grid-cols-2 gap-6">
          <FormField label="Name" fieldKey="name" locked={form.lockedFields.has('name')} onToggleLock={form.toggleLock} fullWidth>
            <TextInput value={form.values.name} onChange={v => form.setField('name', v)} />
          </FormField>
          <FormField label="Overview" fieldKey="overview" locked={form.lockedFields.has('overview')} onToggleLock={form.toggleLock} fullWidth>
            <Textarea value={form.values.overview} onChange={v => form.setField('overview', v)} rows={6} />
          </FormField>
        </div>
        <ImageSelector
          entityType="library-entries"
          entityId={entry.id}
          currentImageUrl={entry.imageUrl}
          onImageSet={() => {
            queryClient.invalidateQueries({ queryKey: ['library-entries', entry.id] })
            onImageSet()
          }}
        />
        <FormField label="Tags" fieldKey="tags" locked={false} onToggleLock={() => {}} fullWidth>
          <TagPicker
            value={filterTagsForModule(currentEntry.tags, 'movies')}
            onAdd={tag => addTag.mutate(tag.id)}
            onRemove={tagId => removeTag.mutate(tagId)}
          />
        </FormField>
        {item && (
          <RelationshipPanel
            entityType="item"
            entityId={item.id}
            contentType={item.contentType}
            people={item.people}
          />
        )}
      </div>
    </EditDrawer>
  )
}

export function MovieDetail() {
  const { id } = useParams<{ id: string }>()
  const [editOpen, setEditOpen] = useState(false)
  const [imgVersion, setImgVersion] = useState(0)

  const { data: entry, isLoading } = useLibraryEntry(id!)
  const { data: itemsPage } = useItems({ libraryEntryId: id!, limit: 1 })
  const item = itemsPage?.data[0]

  if (isLoading) {
    return (
      <div className="px-8 py-10 space-y-6">
        <Skeleton className="h-64 w-full" />
        <div className="flex gap-6">
          <Skeleton className="w-48 h-72 shrink-0" />
          <div className="flex-1 space-y-3">
            <Skeleton className="h-8 w-2/3" />
            <Skeleton className="h-4 w-1/3" />
            <Skeleton className="h-20 w-full" />
          </div>
        </div>
      </div>
    )
  }

  if (!entry) return null

  const performers = item?.people.filter(p => p.role === 'actor' || p.role === 'actress') ?? []
  const visibleTags = filterTagsForModule(entry.tags, 'movies')
  const genreTags = visibleTags.filter(t => t.key === 'genre')
  const productionTags = visibleTags.filter(t => t.key === 'production_company')
  const otherTags = visibleTags.filter(t => t.key !== 'genre' && t.key !== 'production_company')

  return (
    <div>
      {/* Back + Edit */}
      <div className="px-8 pt-6 flex items-center justify-between">
        <Link to="/movies" className="inline-flex items-center gap-1.5 text-sm text-white/40 hover:text-white/70 transition-colors">
          <ArrowLeft size={14} />
          Movies
        </Link>
        <button
          onClick={() => setEditOpen(true)}
          className="inline-flex items-center gap-1.5 text-xs font-medium px-3 py-1.5 rounded-lg border border-white/10 text-white/50 hover:text-white/80 hover:border-white/20 transition-colors"
        >
          <Edit2 size={12} /> Edit
        </button>
      </div>

      {/* Hero */}
      <Hero backdropUrl={entry.imageUrl ?? item?.coverUrl} accent={ACCENT}>
        <div className="flex gap-6 items-end">
          {/* Poster */}
          <div className="shrink-0 w-44 rounded-xl overflow-hidden border border-white/10 shadow-2xl" style={{ aspectRatio: '2/3' }}>
            {entry.imageUrl ? (
              <img src={`${entry.imageUrl}?v=${imgVersion}`} alt={entry.name} className="w-full h-full object-cover" />
            ) : (
              <div className="w-full h-full bg-white/5 flex items-center justify-center">
                <ImageIcon size={40} className="text-white/15" strokeWidth={1} />
              </div>
            )}
          </div>

          {/* Metadata */}
          <div className="flex-1 min-w-0">
            <h1 className="text-3xl font-bold text-white mb-2 leading-tight">{entry.name}</h1>
            <div className="flex flex-wrap items-center gap-2 mb-3">
              {entry.status && <Badge color={ACCENT}>{entry.status}</Badge>}
              {item?.date && (
                <span className="flex items-center gap-1 text-sm text-white/50">
                  <Calendar size={13} />{new Date(item.date).getFullYear()}
                </span>
              )}
              {item?.runtimeSeconds ? (
                <span className="flex items-center gap-1 text-sm text-white/50">
                  <Clock size={13} />{fmtRuntime(item.runtimeSeconds)}
                </span>
              ) : null}
              {item?.mediaFile?.quality && <Badge>{item.mediaFile.quality}</Badge>}
            </div>
            {entry.overview && (
              <p className="text-sm text-white/60 leading-relaxed max-w-2xl line-clamp-4">{entry.overview}</p>
            )}
          </div>
        </div>
      </Hero>

      <div className="px-8 py-8 space-y-10">
        {/* Cast */}
        {performers.length > 0 && (
          <section>
            <h2 className="text-sm font-semibold text-white/40 uppercase tracking-widest mb-4">Cast</h2>
            <div className="grid grid-cols-3 sm:grid-cols-4 md:grid-cols-6 lg:grid-cols-8 gap-3">
              {performers.map(({ person, personId }) => person ? (
                <PersonCard key={personId} person={{ id: personId, name: person.name, sortName: person.sortName, imageUrl: person.imageUrl, overview: '', monitored: false, monitorMode: 'all', aliases: [], externalIds: [], addedAt: '' }} href={`/people/${personId}`} accent={ACCENT} />
              ) : null)}
            </div>
          </section>
        )}

        {/* Genre chips */}
        {genreTags.length > 0 && (
          <section>
            <h2 className="text-sm font-semibold text-white/40 uppercase tracking-widest mb-3">Genre</h2>
            <div className="flex flex-wrap gap-2">
              {genreTags.map(t => <Badge key={t.id} color={ACCENT}>{t.value}</Badge>)}
            </div>
          </section>
        )}

        {/* Production companies */}
        {productionTags.length > 0 && (
          <section>
            <h2 className="text-sm font-semibold text-white/40 uppercase tracking-widest mb-3">Production</h2>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
              {productionTags.map(t => (
                <div key={t.id} className="bg-white/3 rounded-lg p-3">
                  <p className="text-xs text-white/35 mb-0.5">Production Company</p>
                  <p className="text-sm text-white/80 truncate">{t.value}</p>
                </div>
              ))}
            </div>
          </section>
        )}

        {/* Other tags */}
        {otherTags.length > 0 && (
          <section>
            <h2 className="text-sm font-semibold text-white/40 uppercase tracking-widest mb-3">Tags</h2>
            <div className="flex flex-wrap gap-2">
              {otherTags.map(t => <Badge key={t.id}>{t.value}</Badge>)}
            </div>
          </section>
        )}

        {/* File info */}
        {item?.mediaFile && (
          <section>
            <h2 className="text-sm font-semibold text-white/40 uppercase tracking-widest mb-3">File</h2>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
              {[
                { label: 'Path', value: item.mediaFile.path },
                { label: 'Size', value: fmtBytes(item.mediaFile.size) },
                { label: 'Quality', value: item.mediaFile.quality },
                { label: 'Resolution', value: item.mediaFile.resolution },
                { label: 'Codec', value: item.mediaFile.codec },
                { label: 'Container', value: item.mediaFile.container },
              ].filter(r => r.value).map(({ label, value }) => (
                <div key={label} className="bg-white/3 rounded-lg p-3">
                  <p className="text-xs text-white/35 mb-0.5">{label}</p>
                  <p className="text-sm text-white/80 truncate">{value}</p>
                </div>
              ))}
            </div>
          </section>
        )}

        {/* External IDs */}
        {entry.externalIds.length > 0 && (
          <section>
            <h2 className="text-sm font-semibold text-white/40 uppercase tracking-widest mb-3">External IDs</h2>
            <div className="flex flex-wrap gap-2">
              {entry.externalIds.map(e => (
                <span key={e.source} className="text-xs bg-white/5 px-2 py-1 rounded-md text-white/50">
                  {e.source}: {e.value}
                </span>
              ))}
            </div>
          </section>
        )}
      </div>

      {editOpen && (
        <MovieEditDrawer
          entry={entry}
          item={item}
          onClose={() => setEditOpen(false)}
          onImageSet={() => setImgVersion(v => v + 1)}
        />
      )}
    </div>
  )
}
