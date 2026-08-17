import { createQueryOptions, useTransport } from '@connectrpc/connect-query'
import { useQueries } from '@tanstack/react-query'
import { getSelectedImage } from '../gen/purser/domain/v1/image-ImageService_connectquery'

// useGroupImages resolves each given Group's currently selected cover
// art — a Group carries no imageId of its own, same
// useLibraryEntryImages/usePersonImages precedent. The Artist Detail
// Discography tab (#667) fans out one ImageService.GetSelectedImage
// (owner_type="group", image_type="poster") call per album on the page —
// bounded to one artist's groups, the same cheap fan-out useDiscography's
// own ListMusicReleases calls already are.
//
// Returns a groupId -> imageId map, omitting entries with no image so
// AlbumCard's imageId prop stays undefined (its own placeholder-icon
// path) rather than an empty string.
export function useGroupImages(groupIds: string[]): Record<string, string> {
  const transport = useTransport()

  const results = useQueries({
    queries: groupIds.map(groupId => ({
      ...createQueryOptions(getSelectedImage, { ownerType: 'group', ownerId: groupId, imageType: 'poster' }, { transport }),
      staleTime: 5 * 60 * 1000,
      retry: false,
    })),
  })

  const imagesByGroupId: Record<string, string> = {}
  groupIds.forEach((groupId, index) => {
    const image = results[index]?.data?.image
    if (image) {
      imagesByGroupId[groupId] = image.id
    }
  })
  return imagesByGroupId
}
