import { useQueryClient } from '@tanstack/react-query'
import { useAddEntryTag, useRemoveEntryTag, updateLibraryEntry } from '../../../api/library'
import { useEditForm } from '../../../hooks/useEditForm'
import { EditDrawer } from '../EditDrawer'
import { ImageSelector } from '../ImageSelector'
import { FormField } from '../FormField'
import { TextInput } from '../fields/TextInput'
import { Textarea } from '../fields/Textarea'
import { SelectField } from '../fields/SelectField'
import { ExternalIDList } from '../fields/ExternalIDList'
import { TagPicker } from '../fields/TagPicker'
import { RelationshipPanel } from '../RelationshipPanel'
import type { LibraryEntry, EntryStatus, MonitorMode, ExternalID } from '../../../types'

export const STATUS_OPTIONS = [
  { value: 'active',     label: 'Active'     },
  { value: 'continuing', label: 'Continuing' },
  { value: 'ended',      label: 'Ended'      },
]

export const MONITOR_MODE_OPTIONS = [
  { value: 'all',    label: 'All'    },
  { value: 'future', label: 'Future' },
  { value: 'latest', label: 'Latest' },
  { value: 'none',   label: 'None'   },
]

const MONITORED_OPTIONS = [
  { value: 'true',  label: 'Monitored'   },
  { value: 'false', label: 'Unmonitored' },
]

type FormValues = {
  name: string
  sortName: string
  status: EntryStatus
  monitored: string
  monitorMode: MonitorMode
  path: string
  overview: string
  externalIds: ExternalID[]
}

export function initialFormValues(entry: LibraryEntry): FormValues {
  return {
    name:        entry.name,
    sortName:    entry.sortName    ?? '',
    status:      entry.status      ?? 'active',
    monitored:   String(entry.monitored),
    monitorMode: entry.monitorMode ?? 'all',
    path:        entry.path        ?? '',
    overview:    entry.overview    ?? '',
    externalIds: entry.externalIds ?? [],
  }
}

interface LibraryEntryEditorProps {
  entry: LibraryEntry
  onClose: () => void
  onImageSet: () => void
}

export function LibraryEntryEditor({ entry, onClose, onImageSet }: LibraryEntryEditorProps) {
  const queryClient = useQueryClient()
  const addTag    = useAddEntryTag(entry.id)
  const removeTag = useRemoveEntryTag(entry.id)

  const form = useEditForm<FormValues>({
    initial: initialFormValues(entry),
    lockedFields: entry.lockedFields,
    onSubmit: async (values, lockedFields) => {
      const updated = await updateLibraryEntry(entry.id, {
        name:        values.name,
        sortName:    values.sortName,
        status:      values.status,
        monitored:   values.monitored === 'true',
        monitorMode: values.monitorMode,
        path:        values.path,
        overview:    values.overview,
        externalIds: values.externalIds,
        lockedFields,
      })
      queryClient.setQueryData(['library-entries', entry.id], updated)
    },
    onSuccess: onClose,
  })

  const currentEntry = queryClient.getQueryData<LibraryEntry>(['library-entries', entry.id]) ?? entry

  return (
    <EditDrawer title={entry.name} onClose={onClose} onSave={form.submit} saving={form.submitting}>
      <div className="space-y-8">
        <div className="grid grid-cols-2 gap-6">
          <FormField label="Name" fieldKey="name" locked={form.lockedFields.has('name')} onToggleLock={form.toggleLock}>
            <TextInput value={form.values.name} onChange={v => form.setField('name', v)} />
          </FormField>
          <FormField label="Sort Name" fieldKey="sortName" locked={form.lockedFields.has('sortName')} onToggleLock={form.toggleLock}>
            <TextInput value={form.values.sortName} onChange={v => form.setField('sortName', v)} />
          </FormField>

          <FormField label="Status" fieldKey="status" locked={form.lockedFields.has('status')} onToggleLock={form.toggleLock}>
            <SelectField
              value={form.values.status}
              onChange={v => form.setField('status', v as EntryStatus)}
              options={STATUS_OPTIONS}
            />
          </FormField>
          <FormField label="Monitored" fieldKey="monitored" locked={form.lockedFields.has('monitored')} onToggleLock={form.toggleLock}>
            <SelectField
              value={form.values.monitored}
              onChange={v => form.setField('monitored', v)}
              options={MONITORED_OPTIONS}
            />
          </FormField>

          <FormField label="Monitor Mode" fieldKey="monitorMode" locked={form.lockedFields.has('monitorMode')} onToggleLock={form.toggleLock}>
            <SelectField
              value={form.values.monitorMode}
              onChange={v => form.setField('monitorMode', v as MonitorMode)}
              options={MONITOR_MODE_OPTIONS}
            />
          </FormField>
          <FormField label="Path" fieldKey="path" locked={form.lockedFields.has('path')} onToggleLock={form.toggleLock}>
            <TextInput value={form.values.path} onChange={v => form.setField('path', v)} />
          </FormField>

          <FormField label="Overview" fieldKey="overview" locked={form.lockedFields.has('overview')} onToggleLock={form.toggleLock} fullWidth>
            <Textarea value={form.values.overview} onChange={v => form.setField('overview', v)} rows={5} />
          </FormField>

          <FormField label="External IDs" fieldKey="externalIds" locked={false} onToggleLock={() => {}} fullWidth>
            <ExternalIDList
              value={form.values.externalIds}
              onChange={v => form.setField('externalIds', v)}
            />
          </FormField>

          <FormField label="Tags" fieldKey="tags" locked={false} onToggleLock={() => {}} fullWidth>
            <TagPicker
              value={currentEntry.tags}
              onAdd={tag => addTag.mutate(tag.id)}
              onRemove={tagId => removeTag.mutate(tagId)}
            />
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
          people={currentEntry.people}
        />
      </div>
    </EditDrawer>
  )
}
