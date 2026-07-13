// Package entryperson is the datastore-backed adapter for the
// ports.EntryPersonRepository port — a thin instantiation of the shared
// generic composite-key translator in internal/adapters/store. See
// docs/adr/0012-datastore-persistence.md.
package entryperson

import (
	"purser/internal/adapters/datastore"
	"purser/internal/adapters/store"
	"purser/internal/domain"
	"purser/internal/ports"
)

const collection = "entry_person"

// Option customizes a Repository constructed via New.
type Option = store.Option

// WithLogger overrides the default (slog.Default()) logger.
var WithLogger = store.WithLogger

// WithTracerProvider overrides the default (global) TracerProvider.
var WithTracerProvider = store.WithTracerProvider

// WithMeterProvider overrides the default (global) MeterProvider.
var WithMeterProvider = store.WithMeterProvider

// New constructs a named ports.EntryPersonRepository backed by ds.
func New(name string, ds datastore.Datastore, opts ...Option) (ports.EntryPersonRepository, error) {
	return store.NewComposite(name, collection, ds, keyOf, opts...)
}

func keyOf(e *domain.EntryPerson) (string, string, string) {
	return e.LibraryEntryID, e.PersonID, e.Role
}

var _ ports.EntryPersonRepository = (*store.CompositeRepository[domain.EntryPerson])(nil)
