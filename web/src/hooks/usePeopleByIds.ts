import { createQueryOptions, useTransport } from '@connectrpc/connect-query'
import { useQueries } from '@tanstack/react-query'
import type { Person } from '../gen/purser/domain/v1/person_pb'
import { getPerson } from '../gen/purser/domain/v1/person-PersonService_connectquery'

// usePeopleByIds resolves each given Person id to its full Person — the
// Artist Detail Members tab's (#668) join from ListEntryPeople's bare
// person_id rows to a name/PersonCard-renderable record. Same bounded
// per-page fan-out useLibraryEntriesByIds/useItemsByIds already
// established (one Get per id via useQueries + connect-query's
// createQueryOptions — plain useQuery can't be called in a loop).
//
// Returns a personId -> Person map (omitting ids that failed to resolve)
// plus isPending, true while any Get is still in flight.
export function usePeopleByIds(ids: string[]): { peopleById: Record<string, Person>; isPending: boolean } {
  const transport = useTransport()

  const results = useQueries({
    queries: ids.map(id => ({
      ...createQueryOptions(getPerson, { id }, { transport }),
      staleTime: 5 * 60 * 1000,
      retry: false,
    })),
  })

  const peopleById: Record<string, Person> = {}
  ids.forEach((id, index) => {
    const person = results[index]?.data?.person
    if (person) {
      peopleById[id] = person
    }
  })
  return { peopleById, isPending: results.some(result => result.isPending) }
}
