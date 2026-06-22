import { Lock, LockOpen } from 'lucide-react'

interface FormFieldProps {
  label: string
  fieldKey: string
  locked: boolean
  onToggleLock: (field: string) => void
  error?: string
  fullWidth?: boolean
  children: React.ReactNode
}

export function lockButtonClass(locked: boolean): string {
  return locked
    ? 'text-amber-400 transition-colors'
    : 'text-white/20 transition-colors hover:text-white/50'
}

export function FormField({
  label,
  fieldKey,
  locked,
  onToggleLock,
  error,
  fullWidth,
  children,
}: FormFieldProps) {
  return (
    <div className={fullWidth ? 'col-span-2' : ''}>
      <div className="mb-1.5 flex items-center justify-between">
        <label className="text-xs font-medium uppercase tracking-widest text-white/50">
          {label}
        </label>
        <button
          type="button"
          onClick={() => onToggleLock(fieldKey)}
          className={lockButtonClass(locked)}
          title={locked ? 'Unlock field' : 'Lock field'}
        >
          {locked ? <Lock size={13} /> : <LockOpen size={13} />}
        </button>
      </div>
      {children}
      {error && <p className="mt-1 text-xs text-red-400">{error}</p>}
    </div>
  )
}
