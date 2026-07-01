import { X } from 'lucide-react'

interface EditDrawerProps {
  title: string
  onClose: () => void
  onSave: () => void
  saving?: boolean
  action?: React.ReactNode
  children: React.ReactNode
}

export function EditDrawer({ title, onClose, onSave, saving, action, children }: EditDrawerProps) {
  return (
    <>
      <div
        className="fixed inset-0 z-[55] bg-black/50"
        onClick={onClose}
      />
      <div className="fixed inset-y-0 right-0 z-[60] flex w-[65vw] flex-col border-l border-white/10 bg-zinc-900/95 shadow-2xl backdrop-blur-xl">
        <div className="flex items-center gap-4 border-b border-white/10 px-8 py-5">
          <h2 className="flex-1 truncate text-lg font-semibold text-white">{title}</h2>
          {action && <div className="shrink-0">{action}</div>}
          <button
            onClick={onSave}
            disabled={saving}
            className="shrink-0 rounded-lg bg-white/10 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-white/15 disabled:opacity-50"
          >
            {saving ? 'Saving…' : 'Save'}
          </button>
          <button
            onClick={onClose}
            aria-label="Close"
            className="shrink-0 text-white/40 transition-colors hover:text-white/80"
          >
            <X size={20} />
          </button>
        </div>
        <div className="flex-1 overflow-y-auto px-8 py-6">
          {children}
        </div>
      </div>
    </>
  )
}
