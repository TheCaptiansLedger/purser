import { useMutation, useQueryClient } from '@tanstack/react-query'
import { put, del } from './client'
import type { LibraryEntry, Item } from '../types'

export interface EntryPersonInput {
  personId: string
  role: string
  startDate?: string
  endDate?: string
}

export interface ItemPersonInput {
  personId: string
  role: string
}

export function useAddEntryPerson(entryId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: EntryPersonInput) =>
      put<LibraryEntry>(`/library-entries/${entryId}/people`, input),
    onSuccess: (updated) => {
      qc.setQueryData(['library-entries', entryId], updated)
    },
  })
}

export function useRemoveEntryPerson(entryId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ personId, role }: { personId: string; role: string }) =>
      del(`/library-entries/${entryId}/people/${personId}?role=${encodeURIComponent(role)}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['library-entries', entryId] })
    },
  })
}

export function useAddItemPerson(itemId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: ItemPersonInput) =>
      put<Item>(`/items/${itemId}/people`, input),
    onSuccess: (updated) => {
      qc.setQueryData(['items', itemId], updated)
    },
  })
}

export function useRemoveItemPerson(itemId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ personId, role }: { personId: string; role: string }) =>
      del(`/items/${itemId}/people/${personId}?role=${encodeURIComponent(role)}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['items', itemId] })
    },
  })
}
