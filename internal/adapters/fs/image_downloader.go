package fs

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"purser/pkg/httpclient"
	"strings"
)

// ImageDownloader fetches remote images over HTTP and writes them to the
// sharded media directory layout managed by this package.
type ImageDownloader struct {
	mediaPath string
	client    *http.Client
}

// NewImageDownloader constructs an ImageDownloader rooted at mediaPath with a
// 30-second HTTP timeout.
func NewImageDownloader(mediaPath string) *ImageDownloader {
	return &ImageDownloader{
		mediaPath: mediaPath,
		client:    httpclient.New(),
	}
}

// Download fetches the image at url and writes it under entityType/entityID in
// the configured media directory. Returns the file extension (e.g. ".jpg") on
// success, "" on any error (errors are logged internally).
func (d *ImageDownloader) Download(ctx context.Context, url, entityType, entityID string) string {
	destBase := ImagePath(d.mediaPath, entityType, entityID, "")
	if err := os.MkdirAll(filepath.Dir(destBase), 0o750); err != nil {
		slog.Warn("failed to create image dir", "error", err)
		return ""
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		slog.Warn("failed to build image request", "url", url, "error", err)
		return ""
	}
	resp, err := d.client.Do(req)
	if err != nil {
		slog.Warn("failed to fetch image", "url", url, "error", err)
		return ""
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		slog.Warn("unexpected status fetching image", "url", url, "status", resp.StatusCode)
		return ""
	}
	ext := extFromContentType(resp.Header.Get("Content-Type"))
	dest := filepath.Clean(destBase + ext)
	if !strings.HasPrefix(dest, filepath.Clean(d.mediaPath)) {
		slog.Warn("image path outside media dir", "path", dest)
		return ""
	}
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".img-download-*")
	if err != nil {
		slog.Warn("failed to create temp image file", "dir", filepath.Dir(dest), "error", err)
		return ""
	}
	committed := false
	defer func() {
		_ = tmp.Close()
		if !committed {
			_ = os.Remove(tmp.Name())
		}
	}()
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		slog.Warn("failed to write image", "path", tmp.Name(), "error", err)
		return ""
	}
	if err := tmp.Close(); err != nil {
		slog.Warn("failed to close temp image file", "path", tmp.Name(), "error", err)
		return ""
	}
	committed = true
	if err := os.Rename(tmp.Name(), dest); err != nil {
		slog.Warn("failed to rename image file", "src", tmp.Name(), "dest", dest, "error", err)
		return ""
	}
	slog.Debug("image.downloaded", "url", url, "entity_type", entityType, "entity_id", entityID, "ext", ext)
	return ext
}

func extFromContentType(ct string) string {
	switch {
	case strings.Contains(ct, "jpeg"):
		return ".jpg"
	case strings.Contains(ct, "png"):
		return ".png"
	case strings.Contains(ct, "webp"):
		return ".webp"
	case strings.Contains(ct, "gif"):
		return ".gif"
	case strings.Contains(ct, "svg"):
		return ".svg"
	default:
		return ".jpg"
	}
}
