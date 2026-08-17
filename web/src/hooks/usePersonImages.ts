import { createQueryOptions, useTransport } from '@connectrpc/connect-query'
import { useQueries } from '@tanstack/react-query'
import { getSelectedImage } from '../gen/purser/domain/v1/image-ImageService_connectquery'

// usePersonImages resolves each given Person's currently selected photo —
// a Person carries no imageId of its own (see PersonRef's own comment in
// web/src/types/index.ts), so the People index page (#660) fans out one
// ImageService.GetSelectedImage(owner_type="person", owner_id,
// image_type="photo") call per person on the loaded page. This is the
// same documented, bounded per-page fan-out
// docs/technical/music-web-ui.md accepts for the Library grid's ownership
// ring — "UI composes, server doesn't" — not solved by a new
// batch-lookup RPC. useQueries + connect-query's createQueryOptions is
// the combination connect-query itself documents for this shape (plain
// useQuery can't be called in a loop, per the rules of hooks).
//
// GetSelectedImage, not ListImages(page_size=1): the latter has no
// concept of "current" at all, it's just whichever row a filter happened
// to return first. See internal/service/image.go's GetSelected for what
// "selected" actually means (falls back to the newest-attached image
// until something's been explicitly chosen, so no image ever silently
// disappears from here).
//
// Returns a personId -> imageId map, omitting entries with no image so
// PersonCard's imageId prop stays undefined (its own placeholder-avatar
// path) rather than an empty string.
export function usePersonImages(personIds: string[]): Record<string, string> {
  const transport = useTransport()

  const results = useQueries({
    queries: personIds.map(personId => ({
      ...createQueryOptions(
        getSelectedImage,
        { ownerType: 'person', ownerId: personId, imageType: 'photo' },
        { transport },
      ),
      staleTime: 5 * 60 * 1000,
      retry: false,
    })),
  })

  const imagesByPersonId: Record<string, string> = {}
  personIds.forEach((personId, index) => {
    const image = results[index]?.data?.image
    if (image) {
      imagesByPersonId[personId] = image.id
    }
  })
  return imagesByPersonId
}
