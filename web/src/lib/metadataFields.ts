import type { JsonObject } from '@bufbuild/protobuf'

// stringField/aliasesField — small readers for LibraryEntry.Metadata
// (a google.protobuf.Struct, typed JsonObject client-side). Extracted out
// of ArtistDetail so useRefreshArtistMetadata (#671) can read the same
// current-value shape the facts sidebar already renders, rather than
// re-deriving it.
export function stringField(metadata: JsonObject | undefined, key: string): string | undefined {
  const value = metadata?.[key]
  return typeof value === 'string' && value !== '' ? value : undefined
}

export function aliasesField(metadata: JsonObject | undefined): string[] {
  const value = metadata?.aliases
  return Array.isArray(value) ? value.filter((entry): entry is string => typeof entry === 'string') : []
}
