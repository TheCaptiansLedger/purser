// Package libraryentry is the datastore-backed adapter for the
// ports.LibraryEntryRepository port — a thin instantiation of the shared
// generic translator in internal/adapters/store. See
// docs/adr/0012-datastore-persistence.md.
package libraryentry

import (
	"purser/internal/adapters/datastore"
	"purser/internal/adapters/store"
	"purser/internal/domain"
	"purser/internal/ports"
)

const collection = "library_entry"

// Option customizes a Repository constructed via New.
type Option = store.Option

// WithLogger overrides the default (slog.Default()) logger.
var WithLogger = store.WithLogger

// WithTracerProvider overrides the default (global) TracerProvider.
var WithTracerProvider = store.WithTracerProvider

// WithMeterProvider overrides the default (global) MeterProvider.
var WithMeterProvider = store.WithMeterProvider

// New constructs a named ports.LibraryEntryRepository backed by ds.
func New(name string, ds datastore.Datastore, opts ...Option) (ports.LibraryEntryRepository, error) {
	return store.New(name, collection, ds, idOf, opts...)
}

func idOf(e *domain.LibraryEntry) string { return e.ID }

var _ ports.LibraryEntryRepository = (*store.Repository[domain.LibraryEntry])(nil)
