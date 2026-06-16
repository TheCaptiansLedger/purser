import { useQuery } from '@tanstack/react-query'
import { get, getPage } from './client'
import type { Item, ContentType, ItemStatus } from '../types'

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

export function useItems(filter: ItemFilter = {}) {
  return useQuery({
    queryKey: ['items', filter],
    queryFn: () => getPage<Item>('/items', filter as Record<string, string | number | boolean | undefined>),
  })
}

export function useItem(id: string) {
  return useQuery({
    queryKey: ['items', id],
    queryFn: () => get<Item>(`/items/${id}`),
    enabled: !!id,
  })
}
