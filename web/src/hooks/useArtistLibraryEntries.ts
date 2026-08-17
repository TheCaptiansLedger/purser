import { useInfiniteQuery } from '@connectrpc/connect-query'
import { listLibraryEntries } from '../gen/purser/domain/v1/library_entry-LibraryEntryService_connectquery'

// ARTIST_LIBRARY_PAGE_SIZE — the Music Library page's (#664) grid page
// size. Independent of any other page's own pageSize constant, same
// one-constant-per-call-site precedent as PEOPLE_PAGE_SIZE.
const ARTIST_LIBRARY_PAGE_SIZE = 48

// useArtistLibraryEntries wraps LibraryEntryService.ListLibraryEntries,
// filtered to kind="artist", as a cursor-paginated infinite query — the
// Music Library grid's (#664) "load more" list. kind="artist" alone is
// sufficient: it's the only Kind value the domain enum set defines
// (internal/domain/enums.go), and ListLibraryEntriesRequest has no
// content_type filter to begin with. Both the name search box and the
// "Monitored only" toggle filter client-side over the loaded page — see
// docs/technical/music-web-ui.md, no server-side equivalent exists or is
// being added by this story.
export function useArtistLibraryEntries() {
  return useInfiniteQuery(
    listLibraryEntries,
    { pageSize: ARTIST_LIBRARY_PAGE_SIZE, pageToken: '', kind: 'artist' },
    {
      pageParamKey: 'pageToken',
      getNextPageParam: lastPage => (lastPage.nextPageToken !== '' ? lastPage.nextPageToken : undefined),
    },
  )
}
