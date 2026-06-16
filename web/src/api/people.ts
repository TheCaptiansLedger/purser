import { useQuery } from '@tanstack/react-query'
import { get, getPage } from './client'
import type { Person, ContentType, PersonRole } from '../types'

interface PeopleFilter {
  search?: string
  contentType?: ContentType
  role?: PersonRole
  monitored?: boolean
  limit?: number
  offset?: number
}

export function usePeople(filter: PeopleFilter = {}) {
  return useQuery({
    queryKey: ['people', filter],
    queryFn: () => getPage<Person>('/people', filter as Record<string, string | number | boolean | undefined>),
  })
}

export function usePerson(id: string) {
  return useQuery({
    queryKey: ['people', id],
    queryFn: () => get<Person>(`/people/${id}`),
    enabled: !!id,
  })
}
