import { useQuery } from '@tanstack/react-query'
import { getPage } from './client'
import type { Tag } from '../types'

interface TagsFilter {
  scope?: 'user' | 'metadata'
  key?: string
  contentType?: string  // single or comma-separated: 'adult,jav'
}

export function useTags(filter: TagsFilter = {}) {
  return useQuery({
    queryKey: ['tags', filter],
    queryFn: () => getPage<Tag>('/tags', filter as Record<string, string | undefined>),
  })
}
