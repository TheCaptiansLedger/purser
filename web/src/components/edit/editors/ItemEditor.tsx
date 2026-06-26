import { useQueryClient } from '@tanstack/react-query'
import { updateItem, useAddItemTag, useRemoveItemTag } from '../../../api/items'
import { useContentTypeConfigs } from '../../../api/config'
import { itemPersonRoles, RelationshipPanel } from '../RelationshipPanel'
import { useEditForm } from '../../../hooks/useEditForm'
import { EditDrawer } from '../EditDrawer'
import { FormField } from '../FormField'
import { TextInput } from '../fields/TextInput'
import { Textarea } from '../fields/Textarea'
import { TagPicker } from '../fields/TagPicker'
import type { Item } from '../../../types'

type FormValues = { title: string; overview: string }

export function initialFormValues(item: Item): FormValues {
  return {
    title: item.title,
    overview: item.overview ?? '',
  }
}

interface ItemEditorProps {
  item: Item
  onClose: () => void
  hideTagKeys?: string[]
}

export function ItemEditor({ item, onClose, hideTagKeys = [] }: ItemEditorProps) {
  const queryClient = useQueryClient()
  const addTag = useAddItemTag(item.id)
  const removeTag = useRemoveItemTag(item.id)
  const { data: contentTypeConfigs = [] } = useContentTypeConfigs()
  const roles = itemPersonRoles(contentTypeConfigs, item.contentType)

  const form = useEditForm<FormValues>({
    initial: initialFormValues(item),
    lockedFields: item.lockedFields,
    onSubmit: async (values, lockedFields) => {
      const updated = await updateItem(item.id, { ...values, lockedFields })
      queryClient.setQueryData(['items', item.id], updated)
    },
    onSuccess: onClose,
  })

  const currentItem = queryClient.getQueryData<Item>(['items', item.id]) ?? item

  return (
    <EditDrawer title={item.title} onClose={onClose} onSave={form.submit} saving={form.submitting}>
      <div className="space-y-8">
        <div className="grid grid-cols-2 gap-6">
          <FormField label="Title" fieldKey="title" locked={form.lockedFields.has('title')} onToggleLock={form.toggleLock} fullWidth>
            <TextInput value={form.values.title} onChange={v => form.setField('title', v)} />
          </FormField>
          <FormField label="Overview" fieldKey="overview" locked={form.lockedFields.has('overview')} onToggleLock={form.toggleLock} fullWidth>
            <Textarea value={form.values.overview} onChange={v => form.setField('overview', v)} rows={6} />
          </FormField>
        </div>
        <FormField label="Tags" fieldKey="tags" locked={false} onToggleLock={() => {}} fullWidth>
          <TagPicker
            value={currentItem.tags}
            onAdd={tag => addTag.mutate(tag.id)}
            onRemove={tagId => removeTag.mutate(tagId)}
            hideKeys={hideTagKeys}
          />
        </FormField>
        <RelationshipPanel
          entityType="item"
          entityId={item.id}
          roles={roles}
          people={item.people}
        />
      </div>
    </EditDrawer>
  )
}
