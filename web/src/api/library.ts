import { useQuery } from '@tanstack/react-query'
import { get, getPage } from './client'
import type { LibraryEntry, ContentType, Kind } from '../types'

interface LibraryFilter {
  contentType?: ContentType
  kind?: Kind
  parentId?: string
  monitored?: boolean
  search?: string
  tag?: string
  limit?: number
  offset?: number
}

export function useLibraryEntries(filter: LibraryFilter = {}) {
  return useQuery({
    queryKey: ['library-entries', filter],
    queryFn: () => getPage<LibraryEntry>('/library-entries', filter as Record<string, string | number | boolean | undefined>),
  })
}

export function useLibraryEntry(id: string) {
  return useQuery({
    queryKey: ['library-entries', id],
    queryFn: () => get<LibraryEntry>(`/library-entries/${id}`),
    enabled: !!id,
  })
}

export function useChildren(id: string) {
  return useQuery({
    queryKey: ['library-entries', id, 'children'],
    queryFn: () => getPage<LibraryEntry>(`/library-entries/${id}/children`),
    enabled: !!id,
  })
}
