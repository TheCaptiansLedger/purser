import { useMutation, useQuery } from '@connectrpc/connect-query'
import { getPerson, updatePerson } from '../gen/purser/domain/v1/person-PersonService_connectquery'

// usePerson wraps PersonService.GetPerson — the Person Detail page's
// (#661) one read. enabled: !!id since the route param is technically
// optional to the type system even though the route always supplies one.
export function usePerson(id: string) {
  return useQuery(getPerson, { id }, { enabled: !!id })
}

// useUpdatePersonMutation is a separate hook, not folded into usePerson,
// so a component only subscribes to the mutation state it actually
// renders — same one-hook-per-RPC shape useSettings.ts already
// established. Does not auto-invalidate usePerson's query cache; the
// Person Detail page reconciles its own optimistic state from the
// mutation's response instead (round-trip confirmation, not a refetch).
export function useUpdatePersonMutation() {
  return useMutation(updatePerson)
}
