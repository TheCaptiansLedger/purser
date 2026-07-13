// Package mediafile is the datastore-backed adapter for the
// ports.MediaFileRepository port — a thin instantiation of the shared
// generic translator in internal/adapters/store. See
// docs/adr/0012-datastore-persistence.md.
package mediafile

import (
	"purser/internal/adapters/datastore"
	"purser/internal/adapters/store"
	"purser/internal/domain"
	"purser/internal/ports"
)

const collection = "media_file"

// Option customizes a Repository constructed via New.
type Option = store.Option

// WithLogger overrides the default (slog.Default()) logger.
var WithLogger = store.WithLogger

// WithTracerProvider overrides the default (global) TracerProvider.
var WithTracerProvider = store.WithTracerProvider

// WithMeterProvider overrides the default (global) MeterProvider.
var WithMeterProvider = store.WithMeterProvider

// New constructs a named ports.MediaFileRepository backed by ds.
func New(name string, ds datastore.Datastore, opts ...Option) (ports.MediaFileRepository, error) {
	return store.New(name, collection, ds, idOf, opts...)
}

func idOf(m *domain.MediaFile) string { return m.ID }

var _ ports.MediaFileRepository = (*store.Repository[domain.MediaFile])(nil)
