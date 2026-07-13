// Package item is the datastore-backed adapter for the
// ports.ItemRepository port — a thin instantiation of the shared generic
// translator in internal/adapters/store. See
// docs/adr/0012-datastore-persistence.md.
package item

import (
	"purser/internal/adapters/datastore"
	"purser/internal/adapters/store"
	"purser/internal/domain"
	"purser/internal/ports"
)

const collection = "item"

// Option customizes a Repository constructed via New.
type Option = store.Option

// WithLogger overrides the default (slog.Default()) logger.
var WithLogger = store.WithLogger

// WithTracerProvider overrides the default (global) TracerProvider.
var WithTracerProvider = store.WithTracerProvider

// WithMeterProvider overrides the default (global) MeterProvider.
var WithMeterProvider = store.WithMeterProvider

// New constructs a named ports.ItemRepository backed by ds.
func New(name string, ds datastore.Datastore, opts ...Option) (ports.ItemRepository, error) {
	return store.New(name, collection, ds, idOf, opts...)
}

func idOf(i *domain.Item) string { return i.ID }

var _ ports.ItemRepository = (*store.Repository[domain.Item])(nil)
