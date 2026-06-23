import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { del, get, getPage, patch, post } from './client'
import type { Group, ContentType } from '../types'

export type YearSortDir = 'asc' | 'desc'

export function sortGroupsByYear(groups: Group[], dir: YearSortDir): Group[] {
  return [...groups].sort((a, b) => {
    if (a.year === 0 && b.year === 0) return 0
    if (a.year === 0) return 1
    if (b.year === 0) return -1
    return dir === 'asc' ? a.year - b.year : b.year - a.year
  })
}

export function useGroups(libraryEntryId: string, refetchInterval?: number | false) {
  return useQuery({
    queryKey: ['groups', { libraryEntryId }],
    queryFn: () => getPage<Group>('/groups', { libraryEntryId, limit: 200 }),
    enabled: !!libraryEntryId,
    refetchInterval,
  })
}

interface GroupsFilter {
  contentType?: ContentType
  search?: string
  tag_key?: string
  tag_value?: string
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

export function patchGroup(id: string, body: {
  title?: string
  year?: number
  overview?: string
  monitored?: boolean
  monitorMode?: string
  lockedFields?: string[]
}): Promise<Group> {
  return patch<Group>(`/groups/${id}`, body)
}

export function useAddGroupTag(groupId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (tagId: string) => post<Group>(`/groups/${groupId}/tags`, { tagId }),
    onSuccess: (updated) => {
      qc.setQueryData(['groups', groupId], updated)
    },
  })
}

export function useRemoveGroupTag(groupId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (tagId: string) => del(`/groups/${groupId}/tags/${tagId}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['groups', groupId] })
    },
  })
}
