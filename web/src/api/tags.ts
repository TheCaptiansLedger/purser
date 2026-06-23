import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { getPage, post } from './client'
import type { Tag } from '../types'

interface TagsFilter {
  scope?: 'user' | 'metadata'
  key?: string
  contentType?: string  // single or comma-separated: 'adult,jav'
  limit?: number
}

export function useTags(filter: TagsFilter = {}) {
  return useQuery({
    queryKey: ['tags', filter],
    queryFn: () => getPage<Tag>('/tags', filter as Record<string, string | undefined>),
  })
}

interface CreateTagInput {
  key: string
  value: string
  scope?: 'user' | 'metadata'
}

export function useCreateTag() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: CreateTagInput) => post<Tag>('/tags', input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['tags'] })
    },
  })
}
