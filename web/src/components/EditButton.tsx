const BASE =
  'inline-flex items-center gap-1.5 text-xs font-medium px-3 py-1.5 rounded-lg border border-white/10 text-white/50 hover:text-white/80 hover:border-white/20 transition-colors'

export function editButtonClass(extra?: string): string {
  return extra ? `${BASE} ${extra}` : BASE
}

interface EditButtonProps {
  onClick: () => void
  label?: string
  className?: string
}

export function EditButton({ onClick, label = 'Edit', className }: EditButtonProps) {
  return (
    <button onClick={onClick} className={editButtonClass(className)}>
      {label}
    </button>
  )
}
