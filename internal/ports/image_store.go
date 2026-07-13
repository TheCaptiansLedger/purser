package ports

import (
	"context"
	"io"
)

// ImageStore persists and retrieves raw image bytes. Distinct from
// ImageRepository (which persists only the Image metadata row) per
// docs/adr/0013-image-blob-storage.md — a caller writes bytes here
// first, then uses the returned key to build the Image.URL it passes to
// ImageRepository.
type ImageStore interface {
	// Put stores the bytes read from r under ownerType/id, sniffing the
	// content type to pick a file extension. Returns an opaque key that
	// must be passed to Get/Delete — callers never construct a key
	// themselves.
	Put(ctx context.Context, ownerType, id string, r io.Reader) (key string, err error)

	// Get returns a reader for the bytes stored under key. Returns
	// ErrNotFound if key doesn't exist. The caller must Close it.
	Get(ctx context.Context, key string) (io.ReadCloser, error)

	// Delete removes the bytes stored under key. Returns ErrNotFound if
	// key doesn't exist.
	Delete(ctx context.Context, key string) error
}
