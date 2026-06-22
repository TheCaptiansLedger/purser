import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { ArrowLeft, ImageIcon, User, Edit2 } from 'lucide-react'
import { useQueryClient } from '@tanstack/react-query'
import { useLibraryEntry, updateLibraryEntry } from '../../api/library'
import { useItems } from '../../api/items'
import { useEditForm } from '../../hooks/useEditForm'
import { EditDrawer } from '../../components/edit/EditDrawer'
import { ImageSelector } from '../../components/edit/ImageSelector'
import { FormField } from '../../components/edit/FormField'
import { TextInput } from '../../components/edit/fields/TextInput'
import { Textarea } from '../../components/edit/fields/Textarea'
import { RelationshipPanel } from '../../components/edit/RelationshipPanel'
import { Hero } from '../../components/layout/Hero'
import { Badge } from '../../components/ui/Badge'
import { Skeleton } from '../../components/ui/Skeleton'
import type { Item, LibraryEntry } from '../../types'

const ACCENT = '#f59e0b'

type BookFormValues = { name: string; overview: string }

function BookEditDrawer({ entry, item, onClose, onImageSet }: { entry: LibraryEntry; item?: Item; onClose: () => void; onImageSet: () => void }) {
  const queryClient = useQueryClient()
  const form = useEditForm<BookFormValues>({
    initial: { name: entry.name, overview: entry.overview ?? '' },
    lockedFields: entry.lockedFields,
    onSubmit: async (values, lockedFields) => {
      const updated = await updateLibraryEntry(entry.id, { ...values, lockedFields })
      queryClient.setQueryData(['library-entries', entry.id], updated)
    },
    onSuccess: onClose,
  })

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

export function BookDetail() {
  const { id } = useParams<{ id: string }>()
  const [editOpen, setEditOpen] = useState(false)
  const [imgVersion, setImgVersion] = useState(0)

  const { data: entry, isLoading } = useLibraryEntry(id!)
  const { data: itemsPage } = useItems({ libraryEntryId: id!, limit: 1 })
  const item = itemsPage?.data[0]
  const authors = item?.people.filter(p => p.role === 'author') ?? []

  if (isLoading) return <div className="px-8 py-10"><Skeleton className="h-64 w-full" /></div>
  if (!entry) return null

  return (
    <div>
      <div className="px-8 pt-6 flex items-center justify-between">
        <Link to="/books" className="inline-flex items-center gap-1.5 text-sm text-white/40 hover:text-white/70 transition-colors">
          <ArrowLeft size={14} /> Books
        </Link>
        <button
          onClick={() => setEditOpen(true)}
          className="inline-flex items-center gap-1.5 text-xs font-medium px-3 py-1.5 rounded-lg border border-white/10 text-white/50 hover:text-white/80 hover:border-white/20 transition-colors"
        >
          <Edit2 size={12} /> Edit
        </button>
      </div>

      <Hero backdropUrl={entry.imageUrl} accent={ACCENT}>
        <div className="flex gap-6 items-end">
          {/* Cover */}
          <div className="shrink-0 w-36 rounded-xl overflow-hidden border border-white/10 shadow-2xl" style={{ aspectRatio: '2/3' }}>
            {entry.imageUrl ? (
              <img src={`${entry.imageUrl}?v=${imgVersion}`} alt={entry.name} className="w-full h-full object-cover" />
            ) : (
              <div className="w-full h-full bg-white/5 flex items-center justify-center">
                <ImageIcon size={36} className="text-white/15" strokeWidth={1} />
              </div>
            )}
          </div>

          <div className="flex-1 min-w-0">
            <p className="text-xs font-medium uppercase tracking-widest mb-1" style={{ color: ACCENT }}>Book</p>
            <h1 className="text-3xl font-bold text-white mb-2">{entry.name}</h1>
            {authors.length > 0 && (
              <div className="flex items-center gap-2 mb-3">
                <User size={13} className="text-white/40" />
                <span className="text-sm text-white/60">{authors.map(a => a.person?.name).filter(Boolean).join(', ')}</span>
              </div>
            )}
            {item?.date && (
              <div className="flex flex-wrap items-center gap-2 mb-3">
                <Badge color={ACCENT}>{new Date(item.date).getFullYear()}</Badge>
              </div>
            )}
            {entry.overview && (
              <p className="text-sm text-white/60 leading-relaxed max-w-2xl line-clamp-4">{entry.overview}</p>
            )}
          </div>
        </div>
      </Hero>

      <div className="px-8 py-8 space-y-8">
        {entry.overview && (
          <section>
            <h2 className="text-sm font-semibold text-white/40 uppercase tracking-widest mb-3">Synopsis</h2>
            <p className="text-sm text-white/60 leading-relaxed max-w-3xl">{entry.overview}</p>
          </section>
        )}

        {/* Authors */}
        {authors.length > 0 && (
          <section>
            <h2 className="text-sm font-semibold text-white/40 uppercase tracking-widest mb-3">Authors</h2>
            <div className="flex flex-wrap gap-2">
              {authors.map(({ person, personId }) => (
                <Link
                  key={personId}
                  to={`/people/${personId}`}
                  className="flex items-center gap-2 px-3 py-2 rounded-lg bg-white/4 border border-white/5 hover:bg-white/7 hover:border-white/12 transition-all text-sm text-white/70 hover:text-white/90"
                >
                  {person?.imageUrl ? (
                    <img src={person.imageUrl} alt={person.name} className="w-6 h-6 rounded-full object-cover" />
                  ) : (
                    <User size={14} className="text-white/30" />
                  )}
                  {person?.name ?? personId}
                </Link>
              ))}
            </div>
          </section>
        )}

        {/* Tags */}
        {entry.tags.length > 0 && (
          <section>
            <h2 className="text-sm font-semibold text-white/40 uppercase tracking-widest mb-3">Tags</h2>
            <div className="flex flex-wrap gap-2">
              {entry.tags.map(t => <Badge key={t.id} color={ACCENT}>{t.name}</Badge>)}
            </div>
          </section>
        )}

        {/* Metadata */}
        {entry.metadata && Object.keys(entry.metadata).length > 0 && (
          <section>
            <h2 className="text-sm font-semibold text-white/40 uppercase tracking-widest mb-3">Details</h2>
            <div className="grid grid-cols-2 md:grid-cols-3 gap-3">
              {Object.entries(entry.metadata).map(([k, v]) => (
                <div key={k} className="bg-white/3 rounded-lg p-3">
                  <p className="text-xs text-white/35 mb-0.5 capitalize">{k.replace(/_/g, ' ')}</p>
                  <p className="text-sm text-white/75">{String(v)}</p>
                </div>
              ))}
            </div>
          </section>
        )}
      </div>

      {editOpen && (
        <BookEditDrawer
          entry={entry}
          item={item}
          onClose={() => setEditOpen(false)}
          onImageSet={() => setImgVersion(v => v + 1)}
        />
      )}
    </div>
  )
}
