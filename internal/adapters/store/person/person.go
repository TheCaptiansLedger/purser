// Package person is the datastore-backed adapter for the
// ports.PersonRepository port — a thin, entity-specific instantiation of
// the shared generic translator in internal/adapters/store. See
// docs/adr/0012-datastore-persistence.md.
package person

import (
	"purser/internal/adapters/datastore"
	"purser/internal/adapters/store"
	"purser/internal/domain"
	"purser/internal/ports"
)

const collection = "person"

// Option customizes a Repository constructed via New.
type Option = store.Option

// WithLogger overrides the default (slog.Default()) logger.
var WithLogger = store.WithLogger

// WithTracerProvider overrides the default (global) TracerProvider.
var WithTracerProvider = store.WithTracerProvider

// WithMeterProvider overrides the default (global) MeterProvider.
var WithMeterProvider = store.WithMeterProvider

// New constructs a named ports.PersonRepository backed by ds.
func New(name string, ds datastore.Datastore, opts ...Option) (ports.PersonRepository, error) {
	return store.New(name, collection, ds, idOf, opts...)
}

func idOf(p *domain.Person) string { return p.ID }

var _ ports.PersonRepository = (*store.Repository[domain.Person])(nil)
