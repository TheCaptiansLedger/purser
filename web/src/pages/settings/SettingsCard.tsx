import { useState } from 'react'
import type { Setting, SettingCategory } from '../../types'
import { Toggle } from '../../components/Toggle'
import { TextInput } from '../../components/TextInput'
import { SecretInput } from '../../components/SecretInput'
import { ListInput } from '../../components/ListInput'
import { LockBadge } from '../../components/LockBadge'
import { categoryLabel } from './settingsCategory'
import { encodeSettingValue, parseSettingValue } from './settingValue'
import type { ParsedSettingValue } from './settingValue'

export interface SettingsCardProps {
  category: SettingCategory
  settings: Setting[]
  // onSave batches every dirty key in this card into one UpdateSettings
  // call (docs/adr/0028-layered-settings.md — one round trip per Save,
  // not per keystroke). Rejecting the returned promise leaves drafts in
  // place instead of discarding the user's edits.
  onSave: (values: Record<string, string>) => Promise<void>
  onReset: (key: string) => void
  saving: boolean
  resettingKey: string | null
}

// SettingsCard renders one category's fields (#604): locked keys
// read-only with a LockBadge explaining why, unlocked keys editable with
// a per-card batched save and a per-field reset-to-default.
export function SettingsCard({ category, settings, onSave, onReset, saving, resettingKey }: SettingsCardProps) {
  const [drafts, setDrafts] = useState<Record<string, ParsedSettingValue>>({})
  const dirtyKeys = Object.keys(drafts)

  function setDraft(key: string, value: ParsedSettingValue) {
    setDrafts(prev => ({ ...prev, [key]: value }))
  }

  function clearDraft(key: string) {
    setDrafts(prev => {
      if (!(key in prev)) return prev
      const next = { ...prev }
      delete next[key]
      return next
    })
  }

  async function handleSave() {
    const values: Record<string, string> = {}
    for (const key of dirtyKeys) {
      values[key] = encodeSettingValue(drafts[key])
    }
    try {
      await onSave(values)
      setDrafts({})
    } catch {
      // Leave drafts in place on failure so the edits aren't lost —
      // ConfigTab surfaces the mutation error separately.
    }
  }

  function handleReset(key: string) {
    clearDraft(key)
    onReset(key)
  }

  return (
    <section className="bg-surface border border-border rounded-lg p-5 flex flex-col gap-4">
      <h2 className="text-title-md text-text">{categoryLabel(category)}</h2>
      <div className="flex flex-col gap-4">
        {settings.map(setting => (
          <SettingField
            key={setting.key}
            setting={setting}
            draft={setting.key in drafts ? drafts[setting.key] : undefined}
            onChange={value => setDraft(setting.key, value)}
            onReset={() => handleReset(setting.key)}
            resetting={resettingKey === setting.key}
          />
        ))}
      </div>
      {dirtyKeys.length > 0 && (
        <div className="flex justify-end">
          <button
            type="button"
            onClick={handleSave}
            disabled={saving}
            className="h-9 px-4 rounded-lg bg-accent-system text-text text-body font-medium hover:opacity-90 disabled:opacity-50"
          >
            {saving ? 'Saving…' : 'Save changes'}
          </button>
        </div>
      )}
    </section>
  )
}

interface SettingFieldProps {
  setting: Setting
  draft: ParsedSettingValue | undefined
  onChange: (value: ParsedSettingValue) => void
  onReset: () => void
  resetting: boolean
}

// SettingField dispatches one Setting to the right shared field
// component (or the read-only display) — locked keys never reach an
// editable input at all, matching what UpdateSettings would reject
// anyway (docs/adr/0028-layered-settings.md).
function SettingField({ setting, draft, onChange, onReset, resetting }: SettingFieldProps) {
  if (setting.locked) {
    return (
      <div className="flex items-center justify-between gap-3">
        <ReadOnlyField setting={setting} />
        <LockBadge reason={setting.lockReason === 'bootstrap' ? 'bootstrap' : 'operator'} />
      </div>
    )
  }

  const canReset = setting.source === 'db'

  return (
    <div className="flex items-end gap-2">
      <div className="flex-1">
        <SettingInput setting={setting} draft={draft} onChange={onChange} />
      </div>
      {canReset && (
        <button
          type="button"
          onClick={onReset}
          disabled={resetting}
          className="h-9 px-2 rounded-lg text-label text-text-secondary hover:text-text hover:bg-surface-raised disabled:opacity-50"
        >
          {resetting ? 'Resetting…' : 'Reset'}
        </button>
      )}
    </div>
  )
}

interface SettingInputProps {
  setting: Setting
  draft: ParsedSettingValue | undefined
  onChange: (value: ParsedSettingValue) => void
}

// SettingInput picks a field component by the Setting's parsed runtime
// type — GetSettings carries no separate type descriptor, the JSON value
// itself (or, for secrets, the literal masked-placeholder string) is the
// only signal. See web/src/pages/settings/settingValue.ts.
function SettingInput({ setting, draft, onChange }: SettingInputProps) {
  if (setting.secret) {
    // draft, not the parsed placeholder, is what the input shows — a set
    // secret's parsed value is '********', and echoing that into the
    // input would let an un-edited Save round-trip the literal
    // placeholder string as a "new" secret value.
    return (
      <SecretInput
        label={setting.key}
        isSet={parseSettingValue(setting) !== ''}
        value={typeof draft === 'string' ? draft : ''}
        onChange={onChange}
      />
    )
  }

  const value = draft ?? parseSettingValue(setting)

  if (typeof value === 'boolean') {
    return <Toggle label={setting.key} checked={value} onChange={onChange} />
  }
  if (Array.isArray(value)) {
    return <ListInput label={setting.key} value={value} onChange={onChange} />
  }
  if (typeof value === 'number') {
    return <TextInput label={setting.key} value={value} onChange={onChange} type="number" />
  }
  return <TextInput label={setting.key} value={value} onChange={onChange} />
}

function ReadOnlyField({ setting }: { setting: Setting }) {
  const value = parseSettingValue(setting)
  const display = setting.secret ? (value === '' ? 'Not set' : 'Set') : formatReadOnlyValue(value)
  return (
    <div className="flex flex-col gap-1 text-body text-text">
      <span className="text-label text-text-secondary">{setting.key}</span>
      <span>{display}</span>
    </div>
  )
}

function formatReadOnlyValue(value: ParsedSettingValue): string {
  if (Array.isArray(value)) return value.length > 0 ? value.join(', ') : '—'
  if (typeof value === 'boolean') return value ? 'Enabled' : 'Disabled'
  if (value === '') return '—'
  return String(value)
}
