import { useQuery } from '@tanstack/react-query'
import { get, getPage, patch, post } from './client'
import type { MonitorMode, Person, PersonRole } from '../types'

interface PeopleFilter {
  search?: string
  contentType?: string
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

export function updatePerson(id: string, body: Record<string, unknown>) {
  return patch<Person>(`/people/${id}`, body)
}

export interface CreatePersonRequest {
  name: string
  sortName?: string
  overview?: string
  aliases?: string[]
  roles?: PersonRole[]
  monitored: boolean
  monitorMode: MonitorMode
  externalIds?: { source: string; value: string }[]
}

export function createPerson(req: CreatePersonRequest) {
  return post<Person>('/people', req)
}
