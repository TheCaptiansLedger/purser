import { useState } from 'react'
import { X } from 'lucide-react'
import type { CreatePersonRequest } from '../../api/people'
import type { MonitorMode, PersonRole } from '../../types'
import { Toggle } from '../../components/edit/fields/Toggle'
import { AliasList } from '../../components/edit/fields/AliasList'

export interface PersonForm {
  name: string
  sortName: string
  overview: string
  aliases: string[]
  roles: PersonRole[]
  monitored: boolean
  monitorMode: MonitorMode
  externalIds: { source: string; value: string }[]
}

export function blankPersonForm(): PersonForm {
  return {
    name: '',
    sortName: '',
    overview: '',
    aliases: [],
    roles: [],
    monitored: true,
    monitorMode: 'all',
    externalIds: [],
  }
}

export function toCreateRequest(form: PersonForm): CreatePersonRequest {
  return {
    name: form.name,
    sortName: form.sortName || undefined,
    overview: form.overview || undefined,
    aliases: form.aliases.length > 0 ? form.aliases : undefined,
    roles: form.roles.length > 0 ? form.roles : undefined,
    monitored: form.monitored,
    monitorMode: form.monitorMode,
    externalIds: form.externalIds.length > 0 ? form.externalIds : undefined,
  }
}

const ALL_ROLES: { value: PersonRole; label: string }[] = [
  { value: 'performer', label: 'Performer' },
  { value: 'actress', label: 'Actress' },
  { value: 'actor', label: 'Actor' },
  { value: 'director', label: 'Director' },
  { value: 'artist', label: 'Artist' },
  { value: 'producer', label: 'Producer' },
  { value: 'author', label: 'Author' },
]

function toggleRole(roles: PersonRole[], role: PersonRole): PersonRole[] {
  return roles.includes(role) ? roles.filter(r => r !== role) : [...roles, role]
}

interface AddPersonDialogProps {
  open: boolean
  onClose: () => void
  accent: string
  onAdd: (req: CreatePersonRequest) => Promise<void>
}

export function AddPersonDialog({ open, onClose, accent, onAdd }: AddPersonDialogProps) {
  const [form, setForm] = useState<PersonForm>(blankPersonForm)
  const [saving, setSaving] = useState(false)

  if (!open) return null

  function handleClose() {
    setForm(blankPersonForm())
    setSaving(false)
    onClose()
  }

  async function handleAdd() {
    if (!form.name.trim()) return
    setSaving(true)
    try {
      await onAdd(toCreateRequest(form))
      setForm(blankPersonForm())
      onClose()
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4" style={{ background: 'rgba(0,0,0,0.7)' }}>
      <div className="w-full max-w-lg rounded-xl border border-white/10 shadow-2xl flex flex-col" style={{ background: '#0f0f17', maxHeight: '85vh' }}>

        <div className="flex items-center justify-between px-5 py-4 border-b border-white/8">
          <h2 className="text-sm font-semibold text-white">Add Person</h2>
          <button onClick={handleClose} className="text-white/40 hover:text-white/70 transition-colors">
            <X size={16} />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto px-5 py-4 min-h-0 space-y-4">

          <div>
            <label className="block text-xs text-white/40 mb-1">Name <span className="text-red-400">*</span></label>
            <input
              autoFocus
              value={form.name}
              onChange={e => setForm(f => ({ ...f, name: e.target.value }))}
              placeholder="e.g. Jane Smith"
              className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-sm text-white placeholder-white/25 outline-none focus:border-white/20"
            />
          </div>

          <div>
            <label className="block text-xs text-white/40 mb-1">Sort Name</label>
            <input
              value={form.sortName}
              onChange={e => setForm(f => ({ ...f, sortName: e.target.value }))}
              placeholder="e.g. Smith, Jane"
              className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-sm text-white placeholder-white/25 outline-none focus:border-white/20"
            />
          </div>

          <div>
            <label className="block text-xs text-white/40 mb-2">Roles</label>
            <div className="flex flex-wrap gap-2">
              {ALL_ROLES.map(({ value, label }) => {
                const active = form.roles.includes(value)
                return (
                  <button
                    key={value}
                    type="button"
                    onClick={() => setForm(f => ({ ...f, roles: toggleRole(f.roles, value) }))}
                    className="px-3 py-1 rounded-full text-xs font-medium border transition-colors"
                    style={active
                      ? { background: accent + '22', borderColor: accent, color: accent }
                      : { background: 'transparent', borderColor: 'rgba(255,255,255,0.12)', color: 'rgba(255,255,255,0.4)' }
                    }
                  >
                    {label}
                  </button>
                )
              })}
            </div>
          </div>

          <div>
            <label className="block text-xs text-white/40 mb-1">Overview</label>
            <textarea
              rows={3}
              value={form.overview}
              onChange={e => setForm(f => ({ ...f, overview: e.target.value }))}
              className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-sm text-white outline-none focus:border-white/20 resize-none"
            />
          </div>

          <div>
            <label className="block text-xs text-white/40 mb-1.5">Aliases</label>
            <AliasList value={form.aliases} onChange={aliases => setForm(f => ({ ...f, aliases }))} />
          </div>

          <div>
            <label className="block text-xs text-white/40 mb-1">Monitor mode</label>
            <select
              value={form.monitorMode}
              onChange={e => setForm(f => ({ ...f, monitorMode: e.target.value as MonitorMode }))}
              className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-sm text-white outline-none focus:border-white/20"
            >
              <option value="all">All</option>
              <option value="future">Future</option>
              <option value="none">None</option>
              <option value="latest">Latest only</option>
            </select>
          </div>

          <label className="flex items-center gap-3 cursor-pointer">
            <Toggle value={form.monitored} onChange={v => setForm(f => ({ ...f, monitored: v }))} />
            <span className="text-sm text-white/70">Monitored</span>
          </label>

        </div>

        <div className="px-5 py-4 border-t border-white/8 flex justify-end">
          <button
            onClick={() => { void handleAdd() }}
            disabled={!form.name.trim() || saving}
            className="px-4 py-1.5 rounded-lg text-sm font-medium text-white transition-colors disabled:opacity-40"
            style={{ background: accent }}
          >
            {saving ? 'Adding…' : 'Add Person'}
          </button>
        </div>
      </div>
    </div>
  )
}
