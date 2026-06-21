package ports

import "context"

// ImageDownloader downloads a remote image and writes it to the media directory.
type ImageDownloader interface {
	Download(ctx context.Context, url, entityType, entityID string) string
}
