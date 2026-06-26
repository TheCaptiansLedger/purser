import { useQueryClient } from '@tanstack/react-query'
import { updatePerson } from '../../../api/people'
import { useEditForm } from '../../../hooks/useEditForm'
import { EditDrawer } from '../EditDrawer'
import { ImageSelector } from '../ImageSelector'
import { FormField } from '../FormField'
import { TextInput } from '../fields/TextInput'
import { Textarea } from '../fields/Textarea'
import { Toggle } from '../fields/Toggle'
import { SelectField } from '../fields/SelectField'
import { AliasList } from '../fields/AliasList'
import { ExternalIDList } from '../fields/ExternalIDList'
import { MetadataFields, toMetadataStrings } from '../fields/MetadataFields'
import type { Person, MonitorMode, ExternalID } from '../../../types'

export const MONITOR_MODE_OPTIONS = [
  { value: 'all',    label: 'All'    },
  { value: 'future', label: 'Future' },
  { value: 'latest', label: 'Latest' },
  { value: 'none',   label: 'None'   },
]

type FormValues = {
  name: string
  sortName: string
  monitored: boolean
  monitorMode: MonitorMode
  aliases: string[]
  overview: string
  metadata: Record<string, string>
  externalIds: ExternalID[]
}

export function initialFormValues(person: Person): FormValues {
  return {
    name:        person.name,
    sortName:    person.sortName    ?? '',
    monitored:   person.monitored,
    monitorMode: person.monitorMode ?? 'all',
    aliases:     person.aliases     ?? [],
    overview:    person.overview    ?? '',
    metadata:    toMetadataStrings(person.metadata ?? {}),
    externalIds: person.externalIds ?? [],
  }
}

interface PersonEditorProps {
  person: Person
  onClose: () => void
  onImageSet?: () => void
}

export function PersonEditor({ person, onClose, onImageSet }: PersonEditorProps) {
  const queryClient = useQueryClient()

  const form = useEditForm<FormValues>({
    initial: initialFormValues(person),
    lockedFields: person.lockedFields,
    onSubmit: async (values, lockedFields) => {
      const mergedMetadata: Record<string, unknown> = {
        ...(person.metadata ?? {}),
        ...values.metadata,
      }
      const updated = await updatePerson(person.id, {
        name:         values.name,
        sortName:     values.sortName,
        monitored:    values.monitored,
        monitorMode:  values.monitorMode,
        aliases:      values.aliases,
        overview:     values.overview,
        metadata:     mergedMetadata,
        externalIds:  values.externalIds,
        lockedFields,
      })
      queryClient.setQueryData(['people', person.id], updated)
    },
    onSuccess: onClose,
  })

  return (
    <EditDrawer title={person.name} onClose={onClose} onSave={form.submit} saving={form.submitting}>
      <div className="space-y-8">
        <div className="grid grid-cols-2 gap-6">
          <FormField label="Name" fieldKey="name" locked={form.lockedFields.has('name')} onToggleLock={form.toggleLock}>
            <TextInput value={form.values.name} onChange={v => form.setField('name', v)} />
          </FormField>
          <FormField label="Sort Name" fieldKey="sortName" locked={form.lockedFields.has('sortName')} onToggleLock={form.toggleLock}>
            <TextInput value={form.values.sortName} onChange={v => form.setField('sortName', v)} />
          </FormField>

          <FormField label="Monitored" fieldKey="monitored" locked={form.lockedFields.has('monitored')} onToggleLock={form.toggleLock}>
            <Toggle value={form.values.monitored} onChange={v => form.setField('monitored', v)} />
          </FormField>
          <FormField label="Monitor Mode" fieldKey="monitorMode" locked={form.lockedFields.has('monitorMode')} onToggleLock={form.toggleLock}>
            <SelectField
              value={form.values.monitorMode}
              onChange={v => form.setField('monitorMode', v as MonitorMode)}
              options={MONITOR_MODE_OPTIONS}
            />
          </FormField>

          <FormField label="Aliases" fieldKey="aliases" locked={form.lockedFields.has('aliases')} onToggleLock={form.toggleLock} fullWidth>
            <AliasList value={form.values.aliases} onChange={v => form.setField('aliases', v)} />
          </FormField>

          <FormField label="Overview" fieldKey="overview" locked={form.lockedFields.has('overview')} onToggleLock={form.toggleLock} fullWidth>
            <Textarea value={form.values.overview} onChange={v => form.setField('overview', v)} rows={5} />
          </FormField>

          {Object.keys(form.values.metadata).length > 0 && (
            <FormField label="Metadata" fieldKey="metadata" locked={false} onToggleLock={() => {}} fullWidth>
              <MetadataFields value={form.values.metadata} onChange={v => form.setField('metadata', v)} />
            </FormField>
          )}

          <FormField label="External IDs" fieldKey="externalIds" locked={false} onToggleLock={() => {}} fullWidth>
            <ExternalIDList value={form.values.externalIds} onChange={v => form.setField('externalIds', v)} />
          </FormField>
        </div>

        <ImageSelector
          entityType="people"
          entityId={person.id}
          currentImageUrl={person.imageUrl}
          onImageSet={() => {
            void queryClient.invalidateQueries({ queryKey: ['people', person.id] })
            onImageSet?.()
          }}
        />
      </div>
    </EditDrawer>
  )
}
