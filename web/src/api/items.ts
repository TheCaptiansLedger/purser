import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { del, get, getPage, patch, post } from './client'
import type { Item, ContentType, ItemStatus } from '../types'

export function updateItem(id: string, body: Record<string, unknown>) {
  return patch<Item>(`/items/${id}`, body)
}

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

export const SORT_OPTIONS = ['date', 'title'] as const
export const SORT_DIR_OPTIONS = ['asc', 'desc'] as const
export type SortField = typeof SORT_OPTIONS[number]
export type SortDir = typeof SORT_DIR_OPTIONS[number]

interface ItemFilter {
  libraryEntryId?: string
  groupId?: string
  contentType?: ContentType
  status?: ItemStatus
  monitored?: boolean
  personId?: string
  search?: string
  sort?: SortField
  sortDir?: SortDir
  tag_key?: string
  tag_value?: string
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

export function useAddItemTag(itemId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (tagId: string) => post<Item>(`/items/${itemId}/tags`, { tagId }),
    onSuccess: (updated) => {
      qc.setQueryData(['items', itemId], updated)
    },
  })
}

export function useRemoveItemTag(itemId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (tagId: string) => del(`/items/${itemId}/tags/${tagId}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['items', itemId] })
    },
  })
}
