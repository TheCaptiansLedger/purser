import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { ArrowLeft, Building2, ImageIcon, Edit2 } from 'lucide-react'
import { useQueryClient } from '@tanstack/react-query'
import { useLibraryEntry, useChildren, updateLibraryEntry } from '../../api/library'
import { useEditForm } from '../../hooks/useEditForm'
import { EditDrawer } from '../../components/edit/EditDrawer'
import { ImageSelector } from '../../components/edit/ImageSelector'
import { FormField } from '../../components/edit/FormField'
import { TextInput } from '../../components/edit/fields/TextInput'
import { Textarea } from '../../components/edit/fields/Textarea'
import { RelationshipPanel } from '../../components/edit/RelationshipPanel'
import { Hero } from '../../components/layout/Hero'
import { EntryCard } from '../../components/media/EntryCard'
import { Skeleton } from '../../components/ui/Skeleton'
import { EmptyState } from '../../components/ui/EmptyState'
import type { LibraryEntry } from '../../types'

const ACCENT = '#f43f5e'

type NetworkFormValues = { name: string; overview: string }

function NetworkEditDrawer({ entry, onClose, onImageSet }: { entry: LibraryEntry; onClose: () => void; onImageSet: () => void }) {
  const queryClient = useQueryClient()
  const form = useEditForm<NetworkFormValues>({
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
        <RelationshipPanel
          entityType="entry"
          entityId={entry.id}
          contentType={entry.contentType}
          kind={entry.kind}
          people={entry.people}
        />
      </div>
    </EditDrawer>
  )
}

export function NetworkDetail() {
  const { id } = useParams<{ id: string }>()
  const [editOpen, setEditOpen] = useState(false)
  const [imgVersion, setImgVersion] = useState(0)

  const { data: entry, isLoading } = useLibraryEntry(id!)
  const { data: childrenPage } = useChildren(id!)
  const studios = childrenPage?.data ?? []

  if (isLoading) return (
    <div className="px-8 py-10 space-y-4">
      <Skeleton className="h-64 w-full" />
      <Skeleton className="h-8 w-48" />
    </div>
  )
  if (!entry) return null

  return (
    <div>
      <div className="px-8 pt-6 flex items-center justify-between">
        <Link to="/afterdark/networks" className="inline-flex items-center gap-1.5 text-sm text-white/40 hover:text-white/70 transition-colors">
          <ArrowLeft size={14} /> Networks
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
          <div className="shrink-0 w-40 rounded-xl overflow-hidden border border-white/10 shadow-2xl" style={{ aspectRatio: '16/9' }}>
            {entry.imageUrl ? (
              <img src={`${entry.imageUrl}?v=${imgVersion}`} alt={entry.name} className="w-full h-full object-contain p-2" />
            ) : (
              <div className="w-full h-full bg-white/5 flex items-center justify-center">
                <ImageIcon size={32} className="text-white/15" strokeWidth={1} />
              </div>
            )}
          </div>
          <div className="flex-1 min-w-0">
            <p className="text-xs font-medium uppercase tracking-widest mb-1" style={{ color: ACCENT }}>
              Network
            </p>
            <h1 className="text-3xl font-bold text-white mb-2 leading-tight">{entry.name}</h1>
            {studios.length > 0 && (
              <p className="text-sm text-white/40">
                {studios.length} studio{studios.length !== 1 ? 's' : ''}
              </p>
            )}
          </div>
        </div>
      </Hero>

      <div className="px-8 py-8">
        {entry.overview && (
          <p className="text-sm text-white/60 leading-relaxed max-w-3xl mb-10">{entry.overview}</p>
        )}

        {!studios.length ? (
          <EmptyState icon={Building2} title="No studios under this network" accent={ACCENT} />
        ) : (
          <section>
            <h2 className="text-xs font-semibold text-white/35 uppercase tracking-widest mb-4 flex items-center gap-2">
              <Building2 size={13} style={{ color: ACCENT }} />
              Studios
            </h2>
            <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-4">
              {studios.map(s => (
                <EntryCard key={s.id} entry={s} href={`/afterdark/studios/${s.id}`} aspect="16/9" accent={ACCENT} />
              ))}
            </div>
          </section>
        )}
      </div>

      {editOpen && (
        <NetworkEditDrawer
          entry={entry}
          onClose={() => setEditOpen(false)}
          onImageSet={() => setImgVersion(v => v + 1)}
        />
      )}
    </div>
  )
}
