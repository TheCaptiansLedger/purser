package main

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"purser/internal/domain"
	"purser/internal/ports"

	"github.com/google/uuid"
)

// imageMetadataGetter is the narrow slice of ports.ImageRepository
// newImageHandler depends on — same DIP convention internal/api/connect
// handlers already follow (e.g. imageBlobService in image_blob.go): depend
// on exactly the method used, not the full port surface.
type imageMetadataGetter interface {
	Get(ctx context.Context, id string) (*domain.Image, error)
}

// imageBytesGetter is the narrow slice of ports.ImageStore newImageHandler
// depends on.
type imageBytesGetter interface {
	Get(ctx context.Context, key string) (io.ReadCloser, error)
}

// newImageHandler serves an Image's raw bytes over a plain, cacheable GET —
// see docs/technical/image-caching-and-serving.md's "Byte serving" section
// and docs/adr/0013-image-blob-storage.md. Registered at
// "GET /media/images/{id}" on the same mux mountWebUI uses, for the same
// reason: a browser <img src>/the lightbox needs a cacheable GET, and a
// Connect unary RPC (POST with a body) can't be an image source.
//
// id is validated for shape (a parseable UUID) before it ever reaches
// imageRepo.Get — defense in depth on top of ImageStore's own
// path-traversal check (internal/adapters/imagestore/local's resolve),
// per this handler's own acceptance criteria.
func newImageHandler(imageRepo imageMetadataGetter, imageStore imageBytesGetter, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	logger = logger.With("component", "cmd.purser.mediaimages")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		id := r.PathValue("id")

		if _, err := uuid.Parse(id); err != nil {
			http.NotFound(w, r)
			return
		}

		img, err := imageRepo.Get(ctx, id)
		if err != nil {
			if errors.Is(err, ports.ErrNotFound) {
				http.NotFound(w, r)
				return
			}
			logger.ErrorContext(ctx, "looking up image", "id", id, "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		rc, err := imageStore.Get(ctx, img.URL)
		if err != nil {
			if errors.Is(err, ports.ErrNotFound) {
				http.NotFound(w, r)
				return
			}
			logger.ErrorContext(ctx, "reading image bytes", "id", id, "key", img.URL, "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		defer func() {
			if err := rc.Close(); err != nil {
				logger.WarnContext(ctx, "closing image reader", "id", id, "key", img.URL, "error", err)
			}
		}()

		// Content-Type is sniffed from the bytes themselves, never trusted
		// from stored/caller-supplied metadata — same reasoning
		// imagestore/local's Put already applies when picking a file
		// extension.
		br := bufio.NewReaderSize(rc, 512)
		sniff, _ := br.Peek(512)
		w.Header().Set("Content-Type", http.DetectContentType(sniff))
		// Images are immutable once keyed — a changed image gets a new
		// Image row and a new ImageStore key, never an in-place overwrite —
		// so aggressive, long-lived caching is safe.
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")

		if _, err := io.Copy(w, br); err != nil {
			logger.WarnContext(ctx, "writing image response body", "id", id, "key", img.URL, "error", err)
		}
	})
}
