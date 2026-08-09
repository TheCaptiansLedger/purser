package ports

import (
	"context"
	"purser/internal/domain"
)

// SettingsRepository is the persistence port for domain.Setting — the
// DB-stored override layer docs/adr/0028-layered-settings.md defines. It
// knows nothing about Viper, precedence, locked/bootstrap keys, or secret
// masking; those are internal/config's and internal/service's concerns.
// This port is a plain key/value store, the same single-ID, unfiltered
// shape as PersonRepository et al.
//
// List uses opaque cursor pagination: a zero-value pageToken starts from
// the beginning; a non-empty nextPageToken is returned whenever more
// results exist, and is empty on the last page.
type SettingsRepository interface {
	Create(ctx context.Context, setting *domain.Setting) error
	Get(ctx context.Context, key string) (*domain.Setting, error)
	Update(ctx context.Context, setting *domain.Setting) error
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, pageSize int, pageToken string) (settings []*domain.Setting, nextPageToken string, err error)
}
