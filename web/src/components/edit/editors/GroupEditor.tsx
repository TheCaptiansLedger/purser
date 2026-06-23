import { useQueryClient } from '@tanstack/react-query'
import { patchGroup, useAddGroupTag, useRemoveGroupTag } from '../../../api/groups'
import { useEditForm } from '../../../hooks/useEditForm'
import { EditDrawer } from '../EditDrawer'
import { ImageSelector } from '../ImageSelector'
import { FormField } from '../FormField'
import { TextInput } from '../fields/TextInput'
import { Textarea } from '../fields/Textarea'
import { TagPicker } from '../fields/TagPicker'
import type { Group } from '../../../types'

type FormValues = { title: string; year: string; overview: string }

export function initialFormValues(group: Group): FormValues {
  return {
    title:    group.title,
    year:     group.year ? String(group.year) : '',
    overview: group.overview ?? '',
  }
}

interface GroupEditorProps {
  group: Group
  onClose: () => void
  onImageSet?: () => void
}

export function GroupEditor({ group, onClose, onImageSet }: GroupEditorProps) {
  const queryClient = useQueryClient()
  const addTag    = useAddGroupTag(group.id)
  const removeTag = useRemoveGroupTag(group.id)

  const form = useEditForm<FormValues>({
    initial: initialFormValues(group),
    lockedFields: group.lockedFields,
    onSubmit: async (values, lockedFields) => {
      const updated = await patchGroup(group.id, {
        title:        values.title,
        year:         values.year ? parseInt(values.year, 10) : undefined,
        overview:     values.overview,
        lockedFields,
      })
      queryClient.setQueryData(['groups', group.id], updated)
      void queryClient.invalidateQueries({ queryKey: ['groups', { libraryEntryId: group.libraryEntryId }] })
    },
    onSuccess: onClose,
  })

  const currentGroup = queryClient.getQueryData<Group>(['groups', group.id]) ?? group

  return (
    <EditDrawer title={group.title} onClose={onClose} onSave={form.submit} saving={form.submitting}>
      <div className="space-y-8">
        <div className="grid grid-cols-2 gap-6">
          <FormField label="Title" fieldKey="title" locked={form.lockedFields.has('title')} onToggleLock={form.toggleLock} fullWidth>
            <TextInput value={form.values.title} onChange={v => form.setField('title', v)} />
          </FormField>

          <FormField label="Year" fieldKey="year" locked={form.lockedFields.has('year')} onToggleLock={form.toggleLock}>
            <TextInput value={form.values.year} onChange={v => form.setField('year', v)} />
          </FormField>

          <FormField label="Overview" fieldKey="overview" locked={form.lockedFields.has('overview')} onToggleLock={form.toggleLock} fullWidth>
            <Textarea value={form.values.overview} onChange={v => form.setField('overview', v)} rows={6} />
          </FormField>

          <FormField label="Tags" fieldKey="tags" locked={false} onToggleLock={() => {}} fullWidth>
            <TagPicker
              value={currentGroup.tags}
              onAdd={tag => addTag.mutate(tag.id)}
              onRemove={tagId => removeTag.mutate(tagId)}
            />
          </FormField>
        </div>

        <ImageSelector
          entityType="groups"
          entityId={group.id}
          currentImageUrl={group.coverUrl}
          onImageSet={() => {
            queryClient.invalidateQueries({ queryKey: ['groups', group.id] })
            queryClient.invalidateQueries({ queryKey: ['groups', { libraryEntryId: group.libraryEntryId }] })
            onImageSet?.()
          }}
        />
      </div>
    </EditDrawer>
  )
}
