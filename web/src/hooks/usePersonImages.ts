import { createQueryOptions, useTransport } from '@connectrpc/connect-query'
import { useQueries } from '@tanstack/react-query'
import { listImages } from '../gen/purser/domain/v1/image-ImageService_connectquery'

// usePersonImages resolves each given Person's primary photo Image id — a
// Person carries no imageId of its own (see PersonRef's comment in
// web/src/types/index.ts), so the People index page (#660) fans out one
// ImageService.ListImages(owner_type="person", owner_id, pageSize=1) call
// per person on the loaded page. This is the same documented, bounded
// per-page fan-out docs/technical/music-web-ui.md accepts for the Library
// grid's ownership ring — "UI composes, server doesn't" — not solved by a
// new batch-lookup RPC. useQueries + connect-query's createQueryOptions is
// the combination connect-query itself documents for this shape (plain
// useQuery can't be called in a loop, per the rules of hooks).
//
// Returns a personId -> imageId map, omitting entries with no image so
// PersonCard's imageId prop stays undefined (its own placeholder-avatar
// path) rather than an empty string.
export function usePersonImages(personIds: string[]): Record<string, string> {
  const transport = useTransport()

  const results = useQueries({
    queries: personIds.map(personId => ({
      ...createQueryOptions(
        listImages,
        { ownerType: 'person', ownerId: personId, pageSize: 1, pageToken: '' },
        { transport },
      ),
      staleTime: 5 * 60 * 1000,
    })),
  })

  const imagesByPersonId: Record<string, string> = {}
  personIds.forEach((personId, index) => {
    const image = results[index]?.data?.images?.[0]
    if (image) {
      imagesByPersonId[personId] = image.id
    }
  })
  return imagesByPersonId
}
