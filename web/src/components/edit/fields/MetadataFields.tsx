import { TextInput } from './TextInput'

export function toMetadataStrings(meta: Record<string, unknown>): Record<string, string> {
  return Object.fromEntries(
    Object.entries(meta)
      .filter(([, v]) => v === null || v === undefined || typeof v === 'string' || typeof v === 'number' || typeof v === 'boolean')
      .map(([k, v]) => [k, v == null ? '' : String(v)])
  )
}

interface MetadataFieldsProps {
  value: Record<string, string>
  onChange: (v: Record<string, string>) => void
}

export function MetadataFields({ value, onChange }: MetadataFieldsProps) {
  const keys = Object.keys(value)
  if (keys.length === 0) return null

  return (
    <div className="space-y-3">
      {keys.map(key => (
        <div key={key} className="grid grid-cols-[12rem_1fr] items-center gap-4">
          <span className="text-xs font-medium uppercase tracking-widest text-white/35 truncate">
            {key.replace(/_/g, ' ')}
          </span>
          <TextInput value={value[key]} onChange={v => onChange({ ...value, [key]: v })} />
        </div>
      ))}
    </div>
  )
}
