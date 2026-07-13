// Package group is the datastore-backed adapter for the
// ports.GroupRepository port — a thin instantiation of the shared generic
// translator in internal/adapters/store. See
// docs/adr/0012-datastore-persistence.md.
package group

import (
	"purser/internal/adapters/datastore"
	"purser/internal/adapters/store"
	"purser/internal/domain"
	"purser/internal/ports"
)

const collection = "group"

// Option customizes a Repository constructed via New.
type Option = store.Option

// WithLogger overrides the default (slog.Default()) logger.
var WithLogger = store.WithLogger

// WithTracerProvider overrides the default (global) TracerProvider.
var WithTracerProvider = store.WithTracerProvider

// WithMeterProvider overrides the default (global) MeterProvider.
var WithMeterProvider = store.WithMeterProvider

// New constructs a named ports.GroupRepository backed by ds.
func New(name string, ds datastore.Datastore, opts ...Option) (ports.GroupRepository, error) {
	return store.New(name, collection, ds, idOf, opts...)
}

func idOf(g *domain.Group) string { return g.ID }

var _ ports.GroupRepository = (*store.Repository[domain.Group])(nil)
