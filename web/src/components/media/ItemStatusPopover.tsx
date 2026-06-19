import { useState, useRef, useEffect } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { ItemStatusBadge } from './ItemStatusBadge'
import { patchItem } from '../../api/items'
import type { Item, ItemStatus } from '../../types'

interface Action {
  label: string
  status: ItemStatus
}

// getAvailableActions returns user-initiatable transitions for a given status.
// Must stay in sync with legalUserTransitions in internal/domain/status.go.
export function getAvailableActions(status: ItemStatus): Action[] {
  switch (status) {
    case 'wanted':
      return [{ label: 'Skip', status: 'skipped' }]
    case 'missing':
      return [
        { label: 'Mark as Wanted', status: 'wanted' },
        { label: 'Skip', status: 'skipped' },
      ]
    case 'imported':
      return [{ label: 'Mark as Wanted', status: 'wanted' }]
    case 'skipped':
      return [{ label: 'Mark as Wanted', status: 'wanted' }]
    case 'grabbed':
    case 'downloading':
      return [] // pipeline-locked
    default:
      return []
  }
}

interface Props {
  item: Item
}

export function ItemStatusPopover({ item }: Props) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  const queryClient = useQueryClient()

  useEffect(() => {
    if (!open) return
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [open])

  const mutation = useMutation({
    mutationFn: (status: ItemStatus) => patchItem(item.id, { status }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['items'] })
      setOpen(false)
    },
  })

  const actions = getAvailableActions(item.status)

  return (
    <div
      ref={ref}
      className="relative"
      onClick={e => e.preventDefault()}
    >
      <button
        type="button"
        onClick={e => {
          e.preventDefault()
          e.stopPropagation()
          setOpen(v => !v)
        }}
        className="focus:outline-none"
      >
        <ItemStatusBadge status={item.status} monitored={item.monitored} />
      </button>

      {open && actions.length > 0 && (
        <div
          className="absolute bottom-full mb-1.5 right-0 z-50 min-w-[140px] rounded-lg border border-white/10 shadow-xl overflow-hidden"
          style={{ background: 'rgba(20,20,30,0.97)' }}
          onClick={e => { e.preventDefault(); e.stopPropagation() }}
        >
          {actions.map(action => (
            <button
              key={action.status}
              type="button"
              disabled={mutation.isPending}
              onClick={e => {
                e.preventDefault()
                e.stopPropagation()
                mutation.mutate(action.status)
              }}
              className="w-full text-left px-3 py-2 text-xs text-white/70 hover:text-white hover:bg-white/8 transition-colors disabled:opacity-50 disabled:cursor-default"
            >
              {action.label}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
