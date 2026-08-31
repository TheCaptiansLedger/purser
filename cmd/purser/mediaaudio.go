package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"purser/internal/domain"
	"purser/internal/ports"

	"github.com/google/uuid"
)

// mediaFileLister is the narrow slice of ports.MediaFileRepository
// newAudioHandler depends on — same DIP convention as
// imageMetadataGetter/imageBytesGetter in image_blob.go/mediaimages.go:
// depend on exactly the method used, not the full port surface.
type mediaFileLister interface {
	List(ctx context.Context, itemID string, pageSize int, pageToken string) ([]*domain.MediaFile, string, error)
}

// newAudioHandler serves a track's audio bytes over a plain, seekable GET —
// see mediaimages.go's newImageHandler for why this is a plain http.Handler
// rather than a Connect RPC (a browser <audio> element needs a cacheable,
// Range-capable GET; a Connect unary RPC is a POST+body and can't be a
// media source). Registered at "GET /media/audio/{itemId}" on the same mux
// mountWebUI/newImageHandler use. See docs/adr/0011-api-design.md and
// issue #748.
//
// itemId is a Track's Item.ID — the same id the frontend already holds and
// already resolves to a MediaFile via ListMediaFiles({itemId}) for the
// play-icon presence check (useTrackMediaFilePresence). Reusing it here
// avoids plumbing a separate MediaFile.ID through the UI just to name this
// endpoint.
//
// Unlike newImageHandler's io.Copy, this reads the file via os.Open and
// serves it with http.ServeContent, which handles Range/If-Range/
// conditional-GET — required for an <audio> element to seek, not just play
// linearly from the start.
func newAudioHandler(mediaFiles mediaFileLister, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	logger = logger.With("component", "cmd.purser.mediaaudio")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		itemID := r.PathValue("itemId")

		if _, err := uuid.Parse(itemID); err != nil {
			http.NotFound(w, r)
			return
		}

		files, _, err := mediaFiles.List(ctx, itemID, 1, "")
		if err != nil {
			if errors.Is(err, ports.ErrNotFound) {
				http.NotFound(w, r)
				return
			}
			logger.ErrorContext(ctx, "listing media files", "item_id", itemID, "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if len(files) == 0 {
			http.NotFound(w, r)
			return
		}
		path := files[0].Path

		f, err := os.Open(path) //nolint:gosec // path comes from a MediaFile row resolved by itemID, not from the request
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				http.NotFound(w, r)
				return
			}
			logger.ErrorContext(ctx, "opening media file", "item_id", itemID, "path", path, "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		defer func() {
			if err := f.Close(); err != nil {
				logger.WarnContext(ctx, "closing media file", "item_id", itemID, "path", path, "error", err)
			}
		}()

		info, err := f.Stat()
		if err != nil {
			logger.ErrorContext(ctx, "stat-ing media file", "item_id", itemID, "path", path, "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		// http.ServeContent sniffs Content-Type from the file's name/bytes
		// and handles Range/If-Range/conditional-GET itself — the seeking
		// behavior an <audio> element needs that a plain io.Copy doesn't
		// provide.
		http.ServeContent(w, r, path, info.ModTime(), f)
	})
}
