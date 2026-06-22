import { useState, useRef, useEffect } from 'react'
import { Search } from 'lucide-react'
import { usePeople } from '../../api/people'
import type { Person } from '../../types'

export const MIN_SEARCH_LEN = 2

interface PersonSearchInputProps {
  onSelect: (person: Person) => void
  placeholder?: string
}

export function PersonSearchInput({ onSelect, placeholder = 'Search people…' }: PersonSearchInputProps) {
  const [query, setQuery] = useState('')
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  const enabled = query.length >= MIN_SEARCH_LEN
  const { data } = usePeople({ search: query, limit: 10 })
  const results = enabled ? (data?.data ?? []) : []

  useEffect(() => {
    function onClickOutside(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', onClickOutside)
    return () => document.removeEventListener('mousedown', onClickOutside)
  }, [])

  return (
    <div ref={ref} className="relative">
      <div className="flex items-center gap-2 rounded-lg border border-white/10 bg-white/5 px-3 py-2">
        <Search size={14} className="text-white/30 shrink-0" />
        <input
          type="text"
          value={query}
          onChange={e => { setQuery(e.target.value); setOpen(true) }}
          onFocus={() => setOpen(true)}
          placeholder={placeholder}
          className="flex-1 bg-transparent text-sm text-white placeholder-white/30 outline-none"
        />
      </div>
      {open && results.length > 0 && (
        <ul className="absolute z-50 mt-1 w-full rounded-lg border border-white/10 bg-zinc-800 shadow-lg max-h-60 overflow-y-auto">
          {results.map(person => (
            <li key={person.id}>
              <button
                type="button"
                onClick={() => { onSelect(person); setQuery(''); setOpen(false) }}
                className="w-full flex items-center gap-3 px-3 py-2.5 text-sm text-white/80 hover:bg-white/5 text-left transition-colors"
              >
                {person.imageUrl ? (
                  <img src={person.imageUrl} alt={person.name} className="w-6 h-6 rounded-full object-cover shrink-0" />
                ) : (
                  <div className="w-6 h-6 rounded-full bg-white/10 shrink-0" />
                )}
                <span>{person.name}</span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
