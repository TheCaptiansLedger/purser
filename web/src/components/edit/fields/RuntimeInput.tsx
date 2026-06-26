interface RuntimeInputProps {
  value: number
  onChange: (value: number) => void
  disabled?: boolean
}

export function secondsToHMS(total: number): { h: number; m: number; s: number } {
  const h = Math.floor(total / 3600)
  const m = Math.floor((total % 3600) / 60)
  const s = total % 60
  return { h, m, s }
}

export function hmsToSeconds(h: number, m: number, s: number): number {
  return h * 3600 + m * 60 + s
}

export function RuntimeInput({ value, onChange, disabled }: RuntimeInputProps) {
  const { h, m, s } = secondsToHMS(value)

  function part(field: 'h' | 'm' | 's', raw: string) {
    const n = Math.max(0, parseInt(raw, 10) || 0)
    const next = { h, m, s }
    next[field] = n
    onChange(hmsToSeconds(next.h, next.m, next.s))
  }

  const cls = 'w-14 rounded-lg border border-white/10 bg-white/5 px-2 py-2 text-center text-sm text-white transition-colors focus:border-white/25 focus:outline-none disabled:opacity-50'

  return (
    <div className="flex items-center gap-2">
      <input type="number" min={0} value={h} onChange={e => part('h', e.target.value)} disabled={disabled} className={cls} />
      <span className="text-xs text-white/40">h</span>
      <input type="number" min={0} max={59} value={m} onChange={e => part('m', e.target.value)} disabled={disabled} className={cls} />
      <span className="text-xs text-white/40">m</span>
      <input type="number" min={0} max={59} value={s} onChange={e => part('s', e.target.value)} disabled={disabled} className={cls} />
      <span className="text-xs text-white/40">s</span>
    </div>
  )
}
