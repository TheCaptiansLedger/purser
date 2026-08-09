package ports

import (
	"context"
	"io"
)

// ImageFetcher fetches raw image bytes from a remote URL — the "cache a
// remote image locally" half of image support docs/adr/0013-image-blob-storage.md
// named and deliberately deferred ("a thin, separate piece layered on
// ImageStore, not part of the store itself"). This is that piece.
//
// Deliberately provider-agnostic: it takes an arbitrary URL, not a
// StashDB/ThePornDB-specific identifier, so it's shared infrastructure any
// module's Persister can call for any provider's image link (StashDB,
// ThePornDB today; fanart.tv, TheAudioDB, or a future provider tomorrow) —
// not one adapter per provider the way docs/adr/0027-provider-independence.md
// requires for actual metadata lookups. A caller writes the returned bytes
// via ImageStore.Put, then persists the resulting reference via
// ImageRepository, same two-step order ImageStore's own doc comment
// describes.
type ImageFetcher interface {
	// Fetch issues an HTTP GET against url and returns its body. Returns
	// ErrNotFound on an HTTP 404. The caller must Close the returned
	// reader.
	Fetch(ctx context.Context, url string) (io.ReadCloser, error)
}
