package fs

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"purser/internal/media"
	"strings"
	"time"
)

type ImageDownloader struct {
	mediaPath string
	client    *http.Client
}

func NewImageDownloader(mediaPath string) *ImageDownloader {
	return &ImageDownloader{
		mediaPath: mediaPath,
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (d *ImageDownloader) Download(ctx context.Context, url, entityType, entityID string) string {
	destBase := media.ImagePath(d.mediaPath, entityType, entityID, "")
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
	f, err := os.Create(dest) //nolint:gosec
	if err != nil {
		slog.Warn("failed to create image file", "path", dest, "error", err)
		return ""
	}
	defer func() { _ = f.Close() }()
	if _, err := io.Copy(f, resp.Body); err != nil {
		slog.Warn("failed to write image", "path", dest, "error", err)
		_ = os.Remove(dest)
		return ""
	}
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
