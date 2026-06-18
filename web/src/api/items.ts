import { useQuery } from '@tanstack/react-query'
import { get, getPage } from './client'
import type { Item, ContentType, ItemStatus } from '../types'

export async function patchItem(id: string, patch: { monitored?: boolean; status?: ItemStatus }): Promise<Item> {
  const res = await fetch(`/api/v1/items/${id}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(patch),
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error((err as { error?: string }).error ?? res.statusText)
  }
  return res.json() as Promise<Item>
}

interface ItemFilter {
  libraryEntryId?: string
  groupId?: string
  contentType?: ContentType
  status?: ItemStatus
  monitored?: boolean
  personId?: string
  search?: string
  limit?: number
  offset?: number
}

export function useItems(filter: ItemFilter = {}, refetchInterval?: number | false) {
  return useQuery({
    queryKey: ['items', filter],
    queryFn: () => getPage<Item>('/items', filter as Record<string, string | number | boolean | undefined>),
    refetchInterval,
  })
}

export function useItem(id: string) {
  return useQuery({
    queryKey: ['items', id],
    queryFn: () => get<Item>(`/items/${id}`),
    enabled: !!id,
  })
}
