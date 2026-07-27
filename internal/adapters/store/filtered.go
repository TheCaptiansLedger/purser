package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"purser/internal/adapters/datastore"
	"purser/internal/ports"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const filteredInstrumentationName = "purser/internal/adapters/store/filtered"

// FilteredRepository is the datastore-backed translator shared by every
// single-ID entity whose List filters on a fixed set of independent,
// caller-declared Index fields unrelated to the ID itself — the shape
// internal/adapters/store/image/image.go originally hand-wrote, generalized
// per docs/adr/0012-datastore-persistence.md's addendum reserving exactly
// this shape as the trigger for a third generic. LibraryEntry (Kind,
// ParentID) and Item (LibraryEntryID, ContentType, GroupID) are that second
// and third occurrence.
//
// List itself is not generic over a fixed filter signature — each entity's
// thin wrapper package builds its own filter map (the shape differs per
// entity) and calls List with it.
type FilteredRepository[T any] struct {
	name       string
	collection string
	ds         datastore.Datastore
	idOf       func(*T) string
	indexOf    func(*T) map[string]string

	logger        *slog.Logger
	tracer        trace.Tracer
	meterProvider metric.MeterProvider

	creates metric.Int64Counter
	gets    metric.Int64Counter
	updates metric.Int64Counter
	deletes metric.Int64Counter
	lists   metric.Int64Counter
}

// NewFiltered constructs a named FilteredRepository[T] backed by ds,
// storing documents under collection. idOf extracts T's own ID; indexOf
// extracts the fields List can filter on. Both are injected rather than
// required via methods on T, so this package makes no assumption about a
// domain type's field names or method set.
func NewFiltered[T any](name, collection string, ds datastore.Datastore, idOf func(*T) string, indexOf func(*T) map[string]string, opts ...Option) (*FilteredRepository[T], error) {
	if name == "" {
		return nil, fmt.Errorf("adapters/store: name must not be empty")
	}
	if collection == "" {
		return nil, fmt.Errorf("adapters/store: collection must not be empty")
	}
	if ds == nil {
		return nil, fmt.Errorf("adapters/store: ds must not be nil")
	}
	if idOf == nil {
		return nil, fmt.Errorf("adapters/store: idOf must not be nil")
	}
	if indexOf == nil {
		return nil, fmt.Errorf("adapters/store: indexOf must not be nil")
	}

	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	r := &FilteredRepository[T]{
		name:          name,
		collection:    collection,
		ds:            ds,
		idOf:          idOf,
		indexOf:       indexOf,
		logger:        o.logger.With("component", "adapters.store."+collection, "repository.name", name),
		tracer:        o.tracerProvider.Tracer(filteredInstrumentationName),
		meterProvider: o.meterProvider,
	}

	meter := o.meterProvider.Meter(filteredInstrumentationName)
	var err error
	if r.creates, err = meter.Int64Counter(collection+"_repository.creates", metric.WithDescription(collection+" records created")); err != nil {
		return nil, fmt.Errorf("adapters/store: creating creates counter: %w", err)
	}
	if r.gets, err = meter.Int64Counter(collection+"_repository.gets", metric.WithDescription(collection+" Get calls")); err != nil {
		return nil, fmt.Errorf("adapters/store: creating gets counter: %w", err)
	}
	if r.updates, err = meter.Int64Counter(collection+"_repository.updates", metric.WithDescription(collection+" records updated")); err != nil {
		return nil, fmt.Errorf("adapters/store: creating updates counter: %w", err)
	}
	if r.deletes, err = meter.Int64Counter(collection+"_repository.deletes", metric.WithDescription(collection+" records deleted")); err != nil {
		return nil, fmt.Errorf("adapters/store: creating deletes counter: %w", err)
	}
	if r.lists, err = meter.Int64Counter(collection+"_repository.lists", metric.WithDescription(collection+" List calls")); err != nil {
		return nil, fmt.Errorf("adapters/store: creating lists counter: %w", err)
	}

	r.logger.Info(collection + " repository created")
	return r, nil
}

// Create stores v under its own ID, indexed by indexOf(v). Returns
// ports.ErrConflict if a document with that ID already exists.
func (r *FilteredRepository[T]) Create(ctx context.Context, v *T) error {
	id := r.idOf(v)
	ctx, span := r.tracer.Start(ctx, r.collection+"_repository.create",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String(r.collection+".id", id)))
	defer span.End()

	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("adapters/store: marshal %s %s: %w", r.collection, id, err)
	}

	doc := datastore.Document{Collection: r.collection, ID: id, Data: data, Index: r.indexOf(v)}
	if err := r.ds.Create(ctx, doc); err != nil {
		return err
	}

	r.creates.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))
	r.logger.DebugContext(ctx, r.collection+" created", r.collection+".id", id)
	return nil
}

// Get returns the record stored under id. Returns ports.ErrNotFound if
// none exists.
func (r *FilteredRepository[T]) Get(ctx context.Context, id string) (*T, error) {
	ctx, span := r.tracer.Start(ctx, r.collection+"_repository.get",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String(r.collection+".id", id)))
	defer span.End()

	r.gets.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))

	doc, err := r.ds.Get(ctx, r.collection, id)
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			span.SetAttributes(attribute.Bool(r.collection+".found", false))
		}
		return nil, err
	}

	var v T
	if err := json.Unmarshal(doc.Data, &v); err != nil {
		return nil, fmt.Errorf("adapters/store: unmarshal %s %s: %w", r.collection, id, err)
	}
	return &v, nil
}

// Update replaces the record stored under v's own ID, re-indexed by
// indexOf(v). Returns ports.ErrNotFound if none exists.
func (r *FilteredRepository[T]) Update(ctx context.Context, v *T) error {
	id := r.idOf(v)
	ctx, span := r.tracer.Start(ctx, r.collection+"_repository.update",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String(r.collection+".id", id)))
	defer span.End()

	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("adapters/store: marshal %s %s: %w", r.collection, id, err)
	}

	doc := datastore.Document{Collection: r.collection, ID: id, Data: data, Index: r.indexOf(v)}
	if err := r.ds.Update(ctx, doc); err != nil {
		return err
	}

	r.updates.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))
	r.logger.DebugContext(ctx, r.collection+" updated", r.collection+".id", id)
	return nil
}

// Delete removes the record stored under id. Returns ports.ErrNotFound if
// none exists.
func (r *FilteredRepository[T]) Delete(ctx context.Context, id string) error {
	ctx, span := r.tracer.Start(ctx, r.collection+"_repository.delete",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String(r.collection+".id", id)))
	defer span.End()

	if err := r.ds.Delete(ctx, r.collection, id); err != nil {
		return err
	}

	r.deletes.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))
	r.logger.DebugContext(ctx, r.collection+" deleted", r.collection+".id", id)
	return nil
}

// DeleteBatch removes every record whose ID is in ids, as a single
// atomic Datastore transaction — all succeed or none do. Only entities
// with a real bulk-delete API endpoint call this — see
// docs/adr/0016-bulk-operations.md.
func (r *FilteredRepository[T]) DeleteBatch(ctx context.Context, ids []string) error {
	ctx, span := r.tracer.Start(ctx, r.collection+"_repository.delete_batch",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.Int(r.collection+".count", len(ids))))
	defer span.End()

	if err := r.ds.DeleteBatch(ctx, r.collection, ids); err != nil {
		return err
	}

	r.deletes.Add(ctx, int64(len(ids)), metric.WithAttributes(attribute.String("repository.name", r.name)))
	r.logger.DebugContext(ctx, r.collection+" batch deleted", "count", len(ids))
	return nil
}

// UpdateBatch replaces every record in vs, re-indexed by indexOf, as a
// single atomic Datastore transaction — all succeed or none do. Only
// entities with a real bulk-update API endpoint call this — see
// docs/adr/0016-bulk-operations.md.
func (r *FilteredRepository[T]) UpdateBatch(ctx context.Context, vs []*T) error {
	ctx, span := r.tracer.Start(ctx, r.collection+"_repository.update_batch",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.Int(r.collection+".count", len(vs))))
	defer span.End()

	docs := make([]datastore.Document, 0, len(vs))
	for _, v := range vs {
		id := r.idOf(v)
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("adapters/store: marshal %s %s: %w", r.collection, id, err)
		}
		docs = append(docs, datastore.Document{Collection: r.collection, ID: id, Data: data, Index: r.indexOf(v)})
	}

	if err := r.ds.UpdateBatch(ctx, docs); err != nil {
		return err
	}

	r.updates.Add(ctx, int64(len(vs)), metric.WithAttributes(attribute.String("repository.name", r.name)))
	r.logger.DebugContext(ctx, r.collection+" batch updated", "count", len(vs))
	return nil
}

// List returns records in this collection matching filter (a non-empty
// filter restricts results to documents whose Index matches every entry;
// nil/empty means unfiltered), cursor-paginated.
func (r *FilteredRepository[T]) List(ctx context.Context, filter map[string]string, pageSize int, pageToken string) ([]*T, string, error) {
	ctx, span := r.tracer.Start(ctx, r.collection+"_repository.list",
		trace.WithAttributes(attribute.String("repository.name", r.name)))
	defer span.End()

	docs, nextToken, err := r.ds.List(ctx, r.collection, filter, pageSize, pageToken)
	if err != nil {
		return nil, "", err
	}

	records := make([]*T, 0, len(docs))
	for _, doc := range docs {
		var v T
		if err := json.Unmarshal(doc.Data, &v); err != nil {
			return nil, "", fmt.Errorf("adapters/store: unmarshal %s %s: %w", r.collection, doc.ID, err)
		}
		records = append(records, &v)
	}

	r.lists.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))
	r.logger.DebugContext(ctx, r.collection+" list", "count", len(records), "next_page_token", nextToken)
	return records, nextToken, nil
}

// Logger returns the logger this FilteredRepository[T] was constructed
// with. For a hand-written adapter that embeds a FilteredRepository[T] for
// part of its shape (see internal/adapters/store/music) and adds its own
// instrumentation around it, reusing this avoids re-deriving options from
// the same opts list a second time with a different (and possibly
// inconsistent) result — see store.Repository[T].Logger.
func (r *FilteredRepository[T]) Logger() *slog.Logger { return r.logger }

// Tracer returns the tracer this FilteredRepository[T] was constructed
// with — see Logger.
func (r *FilteredRepository[T]) Tracer() trace.Tracer { return r.tracer }

// MeterProvider returns the MeterProvider this FilteredRepository[T] was
// constructed with — see Logger.
func (r *FilteredRepository[T]) MeterProvider() metric.MeterProvider { return r.meterProvider }

// DocumentID returns the opaque datastore.Document ID v's own ID (idOf(v))
// is stored under. A hand-written adapter that must write v's document
// directly via Datastore.CreateBatch alongside a document in a different
// collection (bypassing Create/CreateBatch above — see
// internal/adapters/store/music's MBID reservation fix) uses this instead
// of re-deriving idOf itself.
func (r *FilteredRepository[T]) DocumentID(v *T) string { return r.idOf(v) }

// IndexOf returns the Index map v's document is stored under, per this
// FilteredRepository[T]'s indexOf — see DocumentID.
func (r *FilteredRepository[T]) IndexOf(v *T) map[string]string { return r.indexOf(v) }
