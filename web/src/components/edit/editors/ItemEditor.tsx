import { useQueryClient } from '@tanstack/react-query'
import { updateItem, useAddItemTag, useRemoveItemTag } from '../../../api/items'
import { useContentTypeConfigs } from '../../../api/config'
import { itemPersonRoles, RelationshipPanel } from '../RelationshipPanel'
import { useEditForm } from '../../../hooks/useEditForm'
import { EditDrawer } from '../EditDrawer'
import { FormField } from '../FormField'
import { TextInput } from '../fields/TextInput'
import { Textarea } from '../fields/Textarea'
import { DateInput } from '../fields/DateInput'
import { RuntimeInput } from '../fields/RuntimeInput'
import { TagPicker } from '../fields/TagPicker'
import { Toggle } from '../fields/Toggle'
import { fmtBytes } from '../../ui/Runtime'
import type { Item } from '../../../types'

type FormValues = {
  title: string
  overview: string
  date: string
  sequence: string
  monitored: boolean
  runtimeSeconds: number
}

export function initialFormValues(item: Item): FormValues {
  return {
    title: item.title,
    overview: item.overview ?? '',
    date: item.date ?? '',
    sequence: item.sequence ?? '',
    monitored: item.monitored,
    runtimeSeconds: item.runtimeSeconds,
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
      const updated = await updateItem(item.id, {
        title: values.title,
        overview: values.overview,
        ...(values.date ? { date: values.date } : {}),
        ...(values.sequence ? { sequence: values.sequence } : {}),
        monitored: values.monitored,
        runtimeSeconds: values.runtimeSeconds,
        lockedFields,
      })
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

          <FormField label="Date" fieldKey="date" locked={form.lockedFields.has('date')} onToggleLock={form.toggleLock}>
            <DateInput value={form.values.date} onChange={v => form.setField('date', v)} />
          </FormField>

          <FormField label="Sequence" fieldKey="sequence" locked={form.lockedFields.has('sequence')} onToggleLock={form.toggleLock}>
            <TextInput value={form.values.sequence} onChange={v => form.setField('sequence', v)} placeholder="e.g. S01E03" />
          </FormField>

          <FormField label="Runtime" fieldKey="runtimeSeconds" locked={form.lockedFields.has('runtimeSeconds')} onToggleLock={form.toggleLock}>
            <RuntimeInput value={form.values.runtimeSeconds} onChange={v => form.setField('runtimeSeconds', v)} />
          </FormField>

          <FormField label="Monitored" fieldKey="monitored" locked={form.lockedFields.has('monitored')} onToggleLock={form.toggleLock}>
            <Toggle value={form.values.monitored} onChange={v => form.setField('monitored', v)} />
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

        {currentItem.mediaFile && (
          <div className="space-y-3">
            <h3 className="text-xs font-semibold text-white/40 uppercase tracking-widest">Media File</h3>
            <div className="space-y-2 text-xs">
              <div>
                <span className="block text-white/30 mb-0.5">Path</span>
                <span className="text-white/70 font-mono break-all">{currentItem.mediaFile.path}</span>
              </div>
              <div className="grid grid-cols-2 gap-2">
                {currentItem.mediaFile.size > 0 && (
                  <div>
                    <span className="block text-white/30 mb-0.5">Size</span>
                    <span className="text-white/60">{fmtBytes(currentItem.mediaFile.size)}</span>
                  </div>
                )}
                {currentItem.mediaFile.codec && (
                  <div>
                    <span className="block text-white/30 mb-0.5">Codec</span>
                    <span className="text-white/60">{currentItem.mediaFile.codec}</span>
                  </div>
                )}
                {currentItem.mediaFile.osHash && (
                  <div className="col-span-2">
                    <span className="block text-white/30 mb-0.5">OSHash</span>
                    <span className="text-white/60 font-mono break-all">{currentItem.mediaFile.osHash}</span>
                  </div>
                )}
                {currentItem.mediaFile.md5 && (
                  <div className="col-span-2">
                    <span className="block text-white/30 mb-0.5">MD5</span>
                    <span className="text-white/60 font-mono break-all">{currentItem.mediaFile.md5}</span>
                  </div>
                )}
              </div>
            </div>
          </div>
        )}
      </div>
    </EditDrawer>
  )
}
