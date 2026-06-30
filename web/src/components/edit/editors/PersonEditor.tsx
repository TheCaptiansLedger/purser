import { useState } from 'react'
import { Database } from 'lucide-react'
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
import { PersonScrapeDialog } from './PersonScrapeDialog'
import type { Person, PersonRole, MonitorMode, ExternalID, ExternalPerson } from '../../../types'

export const MONITOR_MODE_OPTIONS = [
  { value: 'all',    label: 'All'    },
  { value: 'future', label: 'Future' },
  { value: 'latest', label: 'Latest' },
  { value: 'none',   label: 'None'   },
]

const ALL_ROLES: { value: PersonRole; label: string }[] = [
  { value: 'performer', label: 'Performer' },
  { value: 'actress',   label: 'Actress'   },
  { value: 'actor',     label: 'Actor'     },
  { value: 'director',  label: 'Director'  },
  { value: 'artist',    label: 'Artist'    },
  { value: 'producer',  label: 'Producer'  },
  { value: 'author',    label: 'Author'    },
]

type FormValues = {
  name: string
  sortName: string
  monitored: boolean
  monitorMode: MonitorMode
  aliases: string[]
  roles: PersonRole[]
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
    roles:       person.roles       ?? [],
    overview:    person.overview    ?? '',
    metadata:    toMetadataStrings(person.metadata ?? {}),
    externalIds: person.externalIds ?? [],
  }
}

function mergeScraped(values: FormValues, scraped: ExternalPerson): Partial<FormValues> {
  const newAliases = [
    ...values.aliases,
    ...(scraped.aliases ?? []).filter(a => !values.aliases.includes(a)),
  ]
  const alreadyLinked = values.externalIds.some(
    id => id.source === scraped.source && id.value === scraped.externalId,
  )
  const newExternalIds = alreadyLinked
    ? values.externalIds
    : [...values.externalIds, { source: scraped.source, value: scraped.externalId }]
  const newRoles = scraped.role && !values.roles.includes(scraped.role)
    ? [...values.roles, scraped.role]
    : values.roles

  // Merge scraped metadata: fill blank keys only, never overwrite existing values.
  const newMetadata = { ...values.metadata }
  for (const [k, v] of Object.entries(scraped.metadata ?? {})) {
    if (!newMetadata[k]) newMetadata[k] = String(v)
  }

  return {
    name:        values.name     || scraped.name,
    overview:    values.overview || scraped.overview || '',
    aliases:     newAliases,
    roles:       newRoles,
    externalIds: newExternalIds,
    metadata:    newMetadata,
  }
}

interface PersonEditorProps {
  person: Person
  onClose: () => void
  onImageSet?: () => void
}

export function PersonEditor({ person, onClose, onImageSet }: PersonEditorProps) {
  const queryClient = useQueryClient()
  const [scrapeOpen, setScrapeOpen] = useState(false)

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
        roles:        values.roles,
        overview:     values.overview,
        metadata:     mergedMetadata,
        externalIds:  values.externalIds,
        lockedFields,
      })
      queryClient.setQueryData(['people', person.id], updated)
    },
    onSuccess: onClose,
  })

  function handleScrapeApply(scraped: ExternalPerson) {
    const patch = mergeScraped(form.values, scraped)
    Object.entries(patch).forEach(([key, value]) => {
      form.setField(key as keyof FormValues, value as FormValues[keyof FormValues])
    })
  }

  const scrapeAction = (
    <button
      type="button"
      onClick={() => setScrapeOpen(true)}
      className="flex items-center gap-1.5 rounded-lg border border-white/10 px-3 py-2 text-sm text-white/50 transition-colors hover:border-white/20 hover:text-white/80"
    >
      <Database size={13} />
      Scrape
    </button>
  )

  return (
    <>
      <EditDrawer title={person.name} onClose={onClose} onSave={form.submit} saving={form.submitting} action={scrapeAction}>
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

            <FormField label="Roles" fieldKey="roles" locked={form.lockedFields.has('roles')} onToggleLock={form.toggleLock} fullWidth>
              <div className="flex flex-wrap gap-2">
                {ALL_ROLES.map(({ value, label }) => {
                  const active = form.values.roles.includes(value)
                  return (
                    <button
                      key={value}
                      type="button"
                      onClick={() => {
                        const next = active
                          ? form.values.roles.filter(r => r !== value)
                          : [...form.values.roles, value]
                        form.setField('roles', next)
                      }}
                      className="px-3 py-1 rounded-full text-xs font-medium border transition-colors"
                      style={active
                        ? { background: '#6366f122', borderColor: '#6366f1', color: '#6366f1' }
                        : { background: 'transparent', borderColor: 'rgba(255,255,255,0.12)', color: 'rgba(255,255,255,0.4)' }
                      }
                    >
                      {label}
                    </button>
                  )
                })}
              </div>
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

      <PersonScrapeDialog
        open={scrapeOpen}
        initialQuery={person.name}
        roles={form.values.roles}
        onClose={() => setScrapeOpen(false)}
        onApply={handleScrapeApply}
      />
    </>
  )
}
