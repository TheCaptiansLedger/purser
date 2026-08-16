// Package person is the datastore-backed adapter for the
// ports.PersonRepository port. Create/Get/Update/Delete reuse the shared
// store.FilteredRepository[T] translator, but List can't use its
// Document.Index-based filtering: the name filter is a case-insensitive
// substring match, and Document.Index only supports exact-match lookups —
// see docs/adr/0012-datastore-persistence.md's Person addendum. A non-empty
// name instead scans the collection through FilteredRepository[T].List
// itself, one underlying record at a time, so the returned page and cursor
// stay exact with no dropped or duplicated matches.
package person

import (
	"context"
	"purser/internal/adapters/datastore"
	"purser/internal/adapters/store"
	"purser/internal/domain"
	"purser/internal/ports"
	"strings"
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

// Repository is the datastore-backed ports.PersonRepository adapter.
type Repository struct {
	inner *store.FilteredRepository[domain.Person]
}

var _ ports.PersonRepository = (*Repository)(nil)

// New constructs a named ports.PersonRepository backed by ds.
func New(name string, ds datastore.Datastore, opts ...Option) (*Repository, error) {
	inner, err := store.NewFiltered(name, collection, ds, idOf, indexOf, opts...)
	if err != nil {
		return nil, err
	}
	return &Repository{inner: inner}, nil
}

func idOf(p *domain.Person) string { return p.ID }

// indexOf is empty: Person.List's only filter (name) is a substring match,
// which Document.Index can't express, so nothing is indexed. See the
// package doc comment.
func indexOf(*domain.Person) map[string]string { return nil }

// Create implements ports.PersonRepository.
func (r *Repository) Create(ctx context.Context, p *domain.Person) error {
	return r.inner.Create(ctx, p)
}

// Get implements ports.PersonRepository.
func (r *Repository) Get(ctx context.Context, id string) (*domain.Person, error) {
	return r.inner.Get(ctx, id)
}

// Update implements ports.PersonRepository.
func (r *Repository) Update(ctx context.Context, p *domain.Person) error {
	return r.inner.Update(ctx, p)
}

// Delete implements ports.PersonRepository.
func (r *Repository) Delete(ctx context.Context, id string) error {
	return r.inner.Delete(ctx, id)
}

// List implements ports.PersonRepository. An empty name is the fast,
// unfiltered path (one indexed page scan). A non-empty name is a
// case-insensitive substring match against Person.Name: since that can't
// be answered from Document.Index, this fetches the underlying collection
// one record at a time and accumulates matches until pageSize is reached
// or the collection is exhausted — one-at-a-time is what keeps the
// returned nextPageToken landing exactly after the last record examined,
// so a resumed List neither skips nor re-scans a record. This trades
// throughput for correctness; a name filter over a much larger Person
// collection than this app's self-hosted, single/small-household scale
// (see docs/adr/0014-search-embedded-full-text-index.md's context) is the
// trigger to move this onto a real search index instead of scaling this
// scan further.
func (r *Repository) List(ctx context.Context, name string, pageSize int, pageToken string) ([]*domain.Person, string, error) {
	if name == "" {
		return r.inner.List(ctx, nil, pageSize, pageToken)
	}

	// Mirror datastore.Datastore.List's own "pageSize<=0 defaults to 50"
	// convention here, so a name-filtered List returns the same default
	// page size an unfiltered one would.
	wantMatches := pageSize
	if wantMatches <= 0 {
		wantMatches = 50
	}

	needle := strings.ToLower(name)
	var matches []*domain.Person
	token := pageToken
	for {
		page, next, err := r.inner.List(ctx, nil, 1, token)
		if err != nil {
			return nil, "", err
		}
		token = next
		for _, p := range page {
			if strings.Contains(strings.ToLower(p.Name), needle) {
				matches = append(matches, p)
			}
		}
		if len(matches) >= wantMatches || token == "" {
			break
		}
	}
	return matches, token, nil
}
