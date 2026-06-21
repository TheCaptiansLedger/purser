package ports

import "context"

type ImageDownloader interface {
	Download(ctx context.Context, url, entityType, entityID string) string
}
