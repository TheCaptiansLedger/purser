import { DragReorderList } from './DragReorderList'

export interface PriorityListInputProps {
  label: string
  value: string[]
  // options is the complete, closed set of valid entries — there is no
  // free-text add, so a saved value can never contain anything outside
  // it. See docs/adr/0027-provider-independence.md's "operator's own
  // configured order, applied consistently" carve-out this list encodes.
  options: { value: string; label: string }[]
  onChange: (value: string[]) => void
  disabled?: boolean
}

// PriorityListInput is the closed-set, reorder-only editor for a
// provider-priority setting (today: afterdark.provider_priority). Built
// on the generic DragReorderList — this file only supplies the
// value<->label lookup and the "no add/remove, only reorder" framing.
export function PriorityListInput({ label, value, options, onChange, disabled }: PriorityListInputProps) {
  function labelOf(v: string): string {
    return options.find(option => option.value === v)?.label ?? v
  }

  return (
    <div className="flex flex-col gap-1.5 text-body text-text">
      <span className="text-label text-text-secondary">{label}</span>
      {value.length > 0 ? (
        <DragReorderList
          items={value}
          getKey={v => v}
          getLabel={labelOf}
          renderItem={labelOf}
          onReorder={onChange}
          ariaLabel={label}
          disabled={disabled}
        />
      ) : (
        <span className="text-text-secondary">—</span>
      )}
    </div>
  )
}
