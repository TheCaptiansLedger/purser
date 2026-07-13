// Package performerprofile is the datastore-backed adapter for the
// ports.PerformerProfileRepository port — a thin instantiation of the
// shared generic translator in internal/adapters/store. See
// docs/adr/0012-datastore-persistence.md.
package performerprofile

import (
	"purser/internal/adapters/datastore"
	"purser/internal/adapters/store"
	"purser/internal/domain/afterdark"
	"purser/internal/ports"
)

const collection = "performer_profile"

// Option customizes a Repository constructed via New.
type Option = store.Option

// WithLogger overrides the default (slog.Default()) logger.
var WithLogger = store.WithLogger

// WithTracerProvider overrides the default (global) TracerProvider.
var WithTracerProvider = store.WithTracerProvider

// WithMeterProvider overrides the default (global) MeterProvider.
var WithMeterProvider = store.WithMeterProvider

// New constructs a named ports.PerformerProfileRepository backed by ds.
// The profile's own key is PersonID, not a separately generated ID.
func New(name string, ds datastore.Datastore, opts ...Option) (ports.PerformerProfileRepository, error) {
	return store.New(name, collection, ds, idOf, opts...)
}

func idOf(p *afterdark.PerformerProfile) string { return p.PersonID }

var _ ports.PerformerProfileRepository = (*store.Repository[afterdark.PerformerProfile])(nil)
