import { useQuery } from '@tanstack/react-query'
import { get, getPage } from './client'
import type { Group, ContentType } from '../types'

export function useGroups(libraryEntryId: string) {
  return useQuery({
    queryKey: ['groups', { libraryEntryId }],
    queryFn: () => getPage<Group>('/groups', { libraryEntryId, limit: 200 }),
    enabled: !!libraryEntryId,
  })
}

interface GroupsFilter {
  contentType?: ContentType
  search?: string
  limit?: number
  offset?: number
}

export function useAllGroups(filter: GroupsFilter = {}) {
  return useQuery({
    queryKey: ['groups', 'all', filter],
    queryFn: () => getPage<Group>('/groups', filter as Record<string, string | number | boolean | undefined>),
  })
}

export function useGroup(id: string) {
  return useQuery({
    queryKey: ['groups', id],
    queryFn: () => get<Group>(`/groups/${id}`),
    enabled: !!id,
  })
}
