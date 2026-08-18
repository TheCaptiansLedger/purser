import { createQueryOptions, useTransport } from '@connectrpc/connect-query'
import { useQueries } from '@tanstack/react-query'
import { getSelectedImage } from '../gen/purser/domain/v1/image-ImageService_connectquery'

// useLibraryEntryImages resolves each given LibraryEntry's currently
// selected poster art — a LibraryEntry carries no imageId of its own,
// same PersonRef precedent usePersonImages already established. The
// Music Library grid (#664) fans out one ImageService.GetSelectedImage
// (owner_type="library_entry", owner_id, image_type="poster") call per
// artist on the loaded page — the same documented, bounded per-page
// fan-out docs/technical/music-web-ui.md accepts for this screen (the
// Discography-ownership fan-out is a separate, larger one — see
// useLibraryOwnership, #670).
//
// Returns a libraryEntryId -> imageId map, omitting entries with no
// image so ArtistCard's imageId prop stays undefined (its own
// placeholder-icon path) rather than an empty string.
export function useLibraryEntryImages(libraryEntryIds: string[]): Record<string, string> {
  const transport = useTransport()

  const results = useQueries({
    queries: libraryEntryIds.map(libraryEntryId => ({
      ...createQueryOptions(
        getSelectedImage,
        { ownerType: 'library_entry', ownerId: libraryEntryId, imageType: 'poster' },
        { transport },
      ),
      staleTime: 5 * 60 * 1000,
      retry: false,
    })),
  })

  const imagesByLibraryEntryId: Record<string, string> = {}
  libraryEntryIds.forEach((libraryEntryId, index) => {
    const image = results[index]?.data?.image
    if (image) {
      imagesByLibraryEntryId[libraryEntryId] = image.id
    }
  })
  return imagesByLibraryEntryId
}
