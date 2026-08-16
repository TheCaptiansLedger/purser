package service

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif"  // side-effect registration, see put's doc comment
	_ "image/jpeg" // side-effect registration, see put's doc comment
	_ "image/png"  // side-effect registration, see put's doc comment
	"io"
	"net/http"
	"purser/internal/domain"
	"purser/internal/ports"

	_ "golang.org/x/image/webp" // side-effect registration, see put's doc comment
)

// blobOwnerType keys ports.ImageStore.Put's sharded path for bytes written
// by ImageBlobService. There is no real owner yet at this point — composing
// "bytes on disk" with "which entity owns them" is the caller's job
// (ImageService.CreateImage, called separately afterward), so this is a
// fixed placeholder, never a real domain.Image.OwnerType. See
// docs/technical/image-caching-and-serving.md.
const blobOwnerType = "unattached"

// maxBlobBytes bounds how much of an input ImageBlobService buffers in
// memory before sniffing it and handing it to ports.ImageStore.Put. Mirrors
// imagestore/local's own default per-image cap
// (docs/adr/0013-image-blob-storage.md) as a generous outer guard — Put
// remains the authoritative size limit for whatever store is actually
// configured; this just keeps a misbehaving remote server (CacheRemoteImage)
// from being read into memory without any bound at all.
const maxBlobBytes = 32<<20 + 1

// BlobResult is what both ImageBlobService operations return: the
// ports.ImageStore key plus everything ImageService.CreateImage needs to
// build a complete domain.Image without re-deriving it. See
// docs/technical/image-caching-and-serving.md.
type BlobResult struct {
	Key         string
	Width       int
	Height      int
	ContentType string
	SizeBytes   int64
}

// ImageBlobService wraps exactly ports.ImageFetcher and ports.ImageStore —
// never ports.ImageRepository. That exclusion is deliberate: composing
// "get bytes onto disk" with "create the metadata row" is the caller's job,
// not this service's, per docs/adr/0013-image-blob-storage.md and
// docs/technical/image-caching-and-serving.md.
type ImageBlobService struct {
	fetcher ports.ImageFetcher
	store   ports.ImageStore
}

// NewImageBlobService constructs an ImageBlobService backed by fetcher and
// store.
func NewImageBlobService(fetcher ports.ImageFetcher, store ports.ImageStore) *ImageBlobService {
	return &ImageBlobService{fetcher: fetcher, store: store}
}

// CacheRemoteImage fetches url via ImageFetcher and writes the resulting
// bytes via ImageStore.Put. Returns ports.ErrNotFound, unwrapped by the
// caller's error mapping to Connect's NOT_FOUND, if the remote server
// returns a 404 — ImageFetcher.Fetch already wraps that case.
func (s *ImageBlobService) CacheRemoteImage(ctx context.Context, url string) (*BlobResult, error) {
	rc, err := s.fetcher.Fetch(ctx, url)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	return s.put(ctx, rc)
}

// UploadImage writes data via ImageStore.Put directly, no fetch.
func (s *ImageBlobService) UploadImage(ctx context.Context, data []byte) (*BlobResult, error) {
	return s.put(ctx, bytes.NewReader(data))
}

// put buffers r (bounded by maxBlobBytes), sniffs content_type
// (http.DetectContentType, the same mechanism imagestore/local's Put uses
// to pick a file extension) and width/height (image.DecodeConfig, via the
// blank-imported jpeg/png/gif/webp decoders — an undecodable format, e.g.
// svg, degrades to 0/0 rather than failing the whole operation) from the
// buffered bytes — never trusted from a caller-supplied hint, per
// docs/technical/image-caching-and-serving.md — then writes the same bytes
// via ImageStore.Put, keyed on a freshly generated id.
func (s *ImageBlobService) put(ctx context.Context, r io.Reader) (*BlobResult, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxBlobBytes))
	if err != nil {
		return nil, fmt.Errorf("service: reading image bytes: %w", err)
	}

	key, err := s.store.Put(ctx, blobOwnerType, domain.NewID(), bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	width, height := 0, 0
	if cfg, _, cfgErr := image.DecodeConfig(bytes.NewReader(data)); cfgErr == nil {
		width, height = cfg.Width, cfg.Height
	}

	return &BlobResult{
		Key:         key,
		Width:       width,
		Height:      height,
		ContentType: http.DetectContentType(data),
		SizeBytes:   int64(len(data)),
	}, nil
}
