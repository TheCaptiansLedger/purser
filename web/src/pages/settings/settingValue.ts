import type { Setting } from '../../types'

export type ParsedSettingValue = string | number | boolean | string[]

// parseSettingValue reads setting.value into its real JS shape.
// Non-secret values are JSON-encoded (per web/src/types/index.ts's
// Setting doc comment); secret values are never JSON — GetSettings
// returns the literal masked placeholder when set, or '' when unset (see
// docs/adr/0028-layered-settings.md's "Secret masking" section) — so a
// secret's parsed value is that literal string, not JSON.parse'd.
export function parseSettingValue(setting: Pick<Setting, 'value' | 'secret'>): ParsedSettingValue {
  if (setting.secret) {
    return setting.value
  }
  return JSON.parse(setting.value === '' ? 'null' : setting.value) as ParsedSettingValue
}

// encodeSettingValue is the write-side inverse of the non-secret branch
// above. Secrets are JSON-encoded on write too — only GetSettings's read
// path special-cases them; internal/config/overlay.go JSON-decodes every
// DB-stored Setting.Value uniformly, secret or not. Callers pass the raw
// new value for both cases; this always JSON-encodes.
export function encodeSettingValue(value: ParsedSettingValue): string {
  return JSON.stringify(value)
}
