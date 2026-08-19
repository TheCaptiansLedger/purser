import { createQueryOptions, useTransport } from '@connectrpc/connect-query'
import { useQueries } from '@tanstack/react-query'
import { listMediaFiles } from '../gen/purser/domain/v1/media_file-MediaFileService_connectquery'
import type { Item } from '../gen/purser/domain/v1/item_pb'
import { ItemStatus } from '../gen/purser/domain/v1/common_pb'

// PRESENCE_PAGE_SIZE — only "does at least one MediaFile exist" needs
// answering per track, never the file list itself, so page size 1 is
// enough.
const PRESENCE_PAGE_SIZE = 1

// useTrackMediaFilePresence answers, per track, "does this track actually
// have a MediaFile backing it" — the second half of #676's disabled-play-
// icon gate (Item.Status=imported AND a MediaFileService.ListMediaFiles
// lookup resolves a file). Fanned out only over *imported* tracks, the
// same bounded, per-parent "useQueries + createQueryOptions, no new bulk
// RPC" shape useDiscography/useGroupTags already establish (ADR 0016's
// bar) — bounded here to one edition's imported tracks, smaller than
// useDiscography's own bound.
export function useTrackMediaFilePresence(tracks: Item[]): Set<string> {
  const transport = useTransport()
  const importedIds = tracks.filter(t => t.status === ItemStatus.IMPORTED).map(t => t.id)

  const results = useQueries({
    queries: importedIds.map(itemId => ({
      ...createQueryOptions(
        listMediaFiles,
        { pageSize: PRESENCE_PAGE_SIZE, pageToken: '', itemId },
        { transport },
      ),
      staleTime: 5 * 60 * 1000,
    })),
  })

  const present = new Set<string>()
  importedIds.forEach((id, index) => {
    if ((results[index]?.data?.mediaFiles?.length ?? 0) > 0) {
      present.add(id)
    }
  })
  return present
}
