import { useState } from 'react'
import { useResetSettingMutation, useSettings, useUpdateSettingsMutation } from '../../hooks/useSettings'
import { settingFromProto } from './fromProto'
import { CATEGORY_ORDER, groupSettingsByCategory } from './settingsCategory'
import { SettingsCard } from './SettingsCard'

// ConfigTab is #604's Config tab: one card per category (see
// settingsCategory.ts), each rendering its Settings via SettingsCard.
// GetSettings is the single source of truth for value/source/lock state —
// this component only fetches, converts (fromProto), groups, and
// dispatches a Save/Reset back to the mutations; SettingsCard owns all
// per-field editing behavior.
export function ConfigTab() {
  const settingsQuery = useSettings()
  const updateSettingsMutation = useUpdateSettingsMutation()
  const resetSettingMutation = useResetSettingMutation()
  const [error, setError] = useState<string | null>(null)

  async function handleSave(values: Record<string, string>) {
    setError(null)
    try {
      await updateSettingsMutation.mutateAsync({ values, updateMask: Object.keys(values) })
      await settingsQuery.refetch()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save settings.')
      throw err
    }
  }

  function handleReset(key: string) {
    setError(null)
    resetSettingMutation.mutate(
      { key },
      {
        onSuccess: () => {
          void settingsQuery.refetch()
        },
        onError: err => {
          setError(err instanceof Error ? err.message : `Failed to reset ${key}.`)
        },
      },
    )
  }

  // Doherty threshold — see docs/design/ux-principles.md#feedback--system-status.
  // A local Connect round trip resolves well under 400ms; a loading
  // indicator here would read as slower, not more informative.
  if (settingsQuery.isPending) {
    return null
  }

  if (settingsQuery.isError) {
    return (
      <p className="text-status-failure text-body" role="alert">
        Couldn't load settings ({settingsQuery.error.message}).
      </p>
    )
  }

  const settings = settingsQuery.data.settings.map(settingFromProto)
  const groups = groupSettingsByCategory(settings)
  const resettingKey = resetSettingMutation.isPending ? (resetSettingMutation.variables?.key ?? null) : null

  return (
    <div className="flex flex-col gap-4">
      {error && (
        <p className="text-status-failure text-body" role="alert">
          {error}
        </p>
      )}
      {CATEGORY_ORDER.filter(category => (groups.get(category) ?? []).length > 0).map(category => (
        <SettingsCard
          key={category}
          category={category}
          settings={groups.get(category) ?? []}
          onSave={handleSave}
          onReset={handleReset}
          saving={updateSettingsMutation.isPending}
          resettingKey={resettingKey}
        />
      ))}
    </div>
  )
}
