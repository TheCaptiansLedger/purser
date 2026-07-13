// Package itemperson is the datastore-backed adapter for the
// ports.ItemPersonRepository port — a thin instantiation of the shared
// generic composite-key translator in internal/adapters/store. See
// docs/adr/0012-datastore-persistence.md.
package itemperson

import (
	"purser/internal/adapters/datastore"
	"purser/internal/adapters/store"
	"purser/internal/domain"
	"purser/internal/ports"
)

const collection = "item_person"

// Option customizes a Repository constructed via New.
type Option = store.Option

// WithLogger overrides the default (slog.Default()) logger.
var WithLogger = store.WithLogger

// WithTracerProvider overrides the default (global) TracerProvider.
var WithTracerProvider = store.WithTracerProvider

// WithMeterProvider overrides the default (global) MeterProvider.
var WithMeterProvider = store.WithMeterProvider

// New constructs a named ports.ItemPersonRepository backed by ds.
func New(name string, ds datastore.Datastore, opts ...Option) (ports.ItemPersonRepository, error) {
	return store.NewComposite(name, collection, ds, keyOf, opts...)
}

func keyOf(i *domain.ItemPerson) (string, string, string) {
	return i.ItemID, i.PersonID, i.Role
}

var _ ports.ItemPersonRepository = (*store.CompositeRepository[domain.ItemPerson])(nil)
