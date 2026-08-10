import { useMutation, useQuery } from '@connectrpc/connect-query'
import {
  getSettings,
  resetSetting,
  updateSettings,
} from '../gen/purser/settings/v1/settings-SettingsService_connectquery'

// useSettings wraps SettingsService.GetSettings (see
// proto/purser/settings/v1/settings.proto) — the Config tab's (#604) one
// read of every config key's current value/source/lock state.
export function useSettings() {
  return useQuery(getSettings, {})
}

// useUpdateSettingsMutation/useResetSettingMutation are separate hooks,
// not folded into useSettings, so a component only subscribes to the
// mutation state it actually renders — same one-hook-per-RPC shape
// useJobs already established. Neither auto-invalidates useSettings'
// query cache; ConfigTab calls the query's own refetch() after a
// successful mutation instead, since it already holds both.
export function useUpdateSettingsMutation() {
  return useMutation(updateSettings)
}

export function useResetSettingMutation() {
  return useMutation(resetSetting)
}
