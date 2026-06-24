import { useState } from 'react'
import { Trash2 } from 'lucide-react'
import { PersonSearchInput } from './PersonSearchInput'
import {
  useAddEntryPerson,
  useRemoveEntryPerson,
  useAddItemPerson,
  useRemoveItemPerson,
} from '../../api/relationships'
import type { ContentTypeConfig, EntryPerson, ItemPerson, KindConfig, Person } from '../../types'

export function itemPersonRoles(configs: ContentTypeConfig[], contentType: string): string[] {
  return configs.find(c => c.contentType === contentType)?.personRoles ?? ['performer']
}

export function entryPersonRoles(configs: KindConfig[], kind: string | undefined): string[] {
  if (!kind) return ['member']
  return configs.find(c => c.kind === kind)?.personRoles ?? ['member']
}

// ── Entry panel ───────────────────────────────────────────────────────────────

interface EntryPanelProps {
  entryId: string
  roles: string[]
  showDates: boolean
  people: EntryPerson[]
}

function EntryPanel({ entryId, roles, showDates, people }: EntryPanelProps) {
  const [role, setRole] = useState(roles[0])
  const [startYear, setStartYear] = useState('')
  const [endYear, setEndYear] = useState('')
  const [confirming, setConfirming] = useState<string | null>(null)

  const add    = useAddEntryPerson(entryId)
  const remove = useRemoveEntryPerson(entryId)

  function handleSelect(person: Person) {
    add.mutate({
      personId:  person.id,
      role,
      startDate: startYear ? `${startYear}-01-01` : undefined,
      endDate:   endYear   ? `${endYear}-01-01`   : undefined,
    })
  }

  function handleRemove(personId: string, personRole: string) {
    const key = `${personId}:${personRole}`
    if (confirming === key) {
      remove.mutate({ personId, role: personRole })
      setConfirming(null)
    } else {
      setConfirming(key)
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex gap-2 flex-wrap">
        <div className="flex-1 min-w-48">
          <PersonSearchInput onSelect={handleSelect} />
        </div>
        <select
          value={role}
          onChange={e => setRole(e.target.value)}
          className="rounded-lg border border-white/10 bg-white/5 px-3 py-2 text-sm text-white/80 outline-none"
        >
          {roles.map(r => (
            <option key={r} value={r}>{r.replace(/_/g, ' ')}</option>
          ))}
        </select>
        {showDates && (
          <>
            <input
              type="number"
              value={startYear}
              onChange={e => setStartYear(e.target.value)}
              placeholder="From"
              className="w-20 rounded-lg border border-white/10 bg-white/5 px-3 py-2 text-sm text-white/80 outline-none placeholder-white/30"
            />
            <input
              type="number"
              value={endYear}
              onChange={e => setEndYear(e.target.value)}
              placeholder="To"
              className="w-20 rounded-lg border border-white/10 bg-white/5 px-3 py-2 text-sm text-white/80 outline-none placeholder-white/30"
            />
          </>
        )}
      </div>

      {people.length === 0 ? (
        <p className="text-sm text-white/25 italic">None linked yet.</p>
      ) : (
        <ul className="space-y-1">
          {people.map(ep => {
            const key = `${ep.personId}:${ep.role}`
            const isConfirming = confirming === key
            return (
              <li
                key={key}
                className="group flex items-center gap-3 rounded-lg px-3 py-2 bg-white/3 hover:bg-white/5 transition-colors"
              >
                {ep.person?.imageUrl ? (
                  <img src={ep.person.imageUrl} alt={ep.person.name} className="w-7 h-7 rounded-full object-cover shrink-0" />
                ) : (
                  <div className="w-7 h-7 rounded-full bg-white/10 shrink-0" />
                )}
                <span className="flex-1 text-sm text-white/80">{ep.person?.name ?? ep.personId}</span>
                <span className="text-xs text-white/35">{ep.role.replace(/_/g, ' ')}</span>
                {ep.startDate && <span className="text-xs text-white/25">{ep.startDate.slice(0, 4)}</span>}
                {ep.endDate   && <span className="text-xs text-white/25">–{ep.endDate.slice(0, 4)}</span>}
                <button
                  onClick={() => handleRemove(ep.personId, ep.role)}
                  className={`ml-auto shrink-0 px-2 py-1 rounded text-xs transition-colors ${
                    isConfirming
                      ? 'bg-red-500/20 text-red-400 hover:bg-red-500/30'
                      : 'opacity-0 group-hover:opacity-100 text-white/30 hover:text-white/70'
                  }`}
                >
                  {isConfirming ? 'Confirm' : <Trash2 size={13} />}
                </button>
              </li>
            )
          })}
        </ul>
      )}
    </div>
  )
}

// ── Item panel ────────────────────────────────────────────────────────────────

interface ItemPanelProps {
  itemId: string
  roles: string[]
  people: ItemPerson[]
}

function ItemPanel({ itemId, roles, people }: ItemPanelProps) {
  const [role, setRole] = useState(roles[0])
  const [confirming, setConfirming] = useState<string | null>(null)

  const add    = useAddItemPerson(itemId)
  const remove = useRemoveItemPerson(itemId)

  function handleSelect(person: Person) {
    add.mutate({ personId: person.id, role })
  }

  function handleRemove(personId: string, personRole: string) {
    const key = `${personId}:${personRole}`
    if (confirming === key) {
      remove.mutate({ personId, role: personRole })
      setConfirming(null)
    } else {
      setConfirming(key)
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex gap-2">
        <div className="flex-1">
          <PersonSearchInput onSelect={handleSelect} />
        </div>
        <select
          value={role}
          onChange={e => setRole(e.target.value)}
          className="rounded-lg border border-white/10 bg-white/5 px-3 py-2 text-sm text-white/80 outline-none"
        >
          {roles.map(r => (
            <option key={r} value={r}>{r.replace(/_/g, ' ')}</option>
          ))}
        </select>
      </div>

      {people.length === 0 ? (
        <p className="text-sm text-white/25 italic">None linked yet.</p>
      ) : (
        <ul className="space-y-1">
          {people.map(ip => {
            const key = `${ip.personId}:${ip.role}`
            const isConfirming = confirming === key
            return (
              <li
                key={key}
                className="group flex items-center gap-3 rounded-lg px-3 py-2 bg-white/3 hover:bg-white/5 transition-colors"
              >
                {ip.person?.imageUrl ? (
                  <img src={ip.person.imageUrl} alt={ip.person.name} className="w-7 h-7 rounded-full object-cover shrink-0" />
                ) : (
                  <div className="w-7 h-7 rounded-full bg-white/10 shrink-0" />
                )}
                <span className="flex-1 text-sm text-white/80">{ip.person?.name ?? ip.personId}</span>
                <span className="text-xs text-white/35">{ip.role.replace(/_/g, ' ')}</span>
                <button
                  onClick={() => handleRemove(ip.personId, ip.role)}
                  className={`ml-auto shrink-0 px-2 py-1 rounded text-xs transition-colors ${
                    isConfirming
                      ? 'bg-red-500/20 text-red-400 hover:bg-red-500/30'
                      : 'opacity-0 group-hover:opacity-100 text-white/30 hover:text-white/70'
                  }`}
                >
                  {isConfirming ? 'Confirm' : <Trash2 size={13} />}
                </button>
              </li>
            )
          })}
        </ul>
      )}
    </div>
  )
}

// ── Public component ──────────────────────────────────────────────────────────

interface RelationshipPanelProps {
  entityType: 'entry' | 'item'
  entityId: string
  roles: string[]
  showDates?: boolean
  people: EntryPerson[] | ItemPerson[]
}

export function RelationshipPanel({ entityType, entityId, roles, showDates, people }: RelationshipPanelProps) {
  return (
    <div>
      <h3 className="text-sm font-semibold text-white/40 uppercase tracking-widest mb-4">Links</h3>
      {entityType === 'entry' ? (
        <EntryPanel
          entryId={entityId}
          roles={roles}
          showDates={showDates ?? false}
          people={people as EntryPerson[]}
        />
      ) : (
        <ItemPanel
          itemId={entityId}
          roles={roles}
          people={people as ItemPerson[]}
        />
      )}
    </div>
  )
}
