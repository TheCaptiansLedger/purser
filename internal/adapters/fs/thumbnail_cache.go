package fs

import (
	"context"
	"purser/internal/ports"
)

type thumbnailCache struct {
	mediaPath string
	dl        *ImageDownloader
}

// NewThumbnailCache returns a ports.ThumbnailCache that fetches remote images
// and writes them under {mediaPath}/thumbnails/ using the sharded layout.
func NewThumbnailCache(mediaPath string) ports.ThumbnailCache {
	return &thumbnailCache{
		mediaPath: mediaPath,
		dl:        NewImageDownloader(mediaPath),
	}
}

func (t *thumbnailCache) Store(ctx context.Context, url, key string) string {
	ext := t.dl.Download(ctx, url, "thumbnails", key)
	if ext == "" {
		return ""
	}
	return ImagePath(t.mediaPath, "thumbnails", key, ext)
}
