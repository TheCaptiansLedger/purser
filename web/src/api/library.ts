import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { del, get, getPage, patch, post } from './client'
import type { LibraryEntry, ContentType, Kind } from '../types'

interface LibraryFilter {
  contentType?: ContentType
  kind?: Kind
  parentId?: string
  personId?: string
  monitored?: boolean
  search?: string
  tag_key?: string
  tag_value?: string
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

export function updateLibraryEntry(id: string, body: Record<string, unknown>) {
  return patch<LibraryEntry>(`/library-entries/${id}`, body)
}

export function useAddEntryTag(entryId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (tagId: string) => post<LibraryEntry>(`/library-entries/${entryId}/tags`, { tagId }),
    onSuccess: (updated) => {
      qc.setQueryData(['library-entries', entryId], updated)
    },
  })
}

export function useRemoveEntryTag(entryId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (tagId: string) => del(`/library-entries/${entryId}/tags/${tagId}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['library-entries', entryId] })
    },
  })
}
