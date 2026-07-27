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

const compositeInstrumentationName = "purser/internal/adapters/store/composite"

// keySeparator joins composite key parts into a single opaque
// datastore.Document ID.
const keySeparator = "\x00"

// CompositeRepository is the datastore-backed translator shared by every
// entity keyed on 3 joined string parts — EntryPerson, ItemPerson,
// ExternalID, and (via a thin type-converting wrapper) TagAssignment. See
// docs/adr/0012-datastore-persistence.md.
//
// Which fields List can filter on is declared per entity by indexOf, not
// hardcoded here — EntryPerson/ItemPerson/ExternalID only ever needed 2 of
// their 3 key parts indexed; TagAssignment needs all 3 independently
// indexed (browse-by-tag vs. show-an-entity's-tags are both real queries).
// List itself takes a filter map built by the caller (each entity's thin
// wrapper), not positional k1/k2 args, so the set of filterable fields is
// entirely up to indexOf.
type CompositeRepository[T any] struct {
	name       string
	collection string
	ds         datastore.Datastore
	keyOf      func(*T) (string, string, string)
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

// NewComposite constructs a named CompositeRepository[T] backed by ds,
// storing documents under collection. keyOf extracts T's 3 key parts;
// indexOf declares which fields (and values) List can filter on. Both are
// injected rather than required via methods on T, so this package makes no
// assumption about a domain type's field names.
func NewComposite[T any](name, collection string, ds datastore.Datastore, keyOf func(*T) (string, string, string), indexOf func(*T) map[string]string, opts ...Option) (*CompositeRepository[T], error) {
	if name == "" {
		return nil, fmt.Errorf("adapters/store: name must not be empty")
	}
	if collection == "" {
		return nil, fmt.Errorf("adapters/store: collection must not be empty")
	}
	if ds == nil {
		return nil, fmt.Errorf("adapters/store: ds must not be nil")
	}
	if keyOf == nil {
		return nil, fmt.Errorf("adapters/store: keyOf must not be nil")
	}
	if indexOf == nil {
		return nil, fmt.Errorf("adapters/store: indexOf must not be nil")
	}

	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	r := &CompositeRepository[T]{
		name:          name,
		collection:    collection,
		ds:            ds,
		keyOf:         keyOf,
		indexOf:       indexOf,
		logger:        o.logger.With("component", "adapters.store."+collection, "repository.name", name),
		tracer:        o.tracerProvider.Tracer(compositeInstrumentationName),
		meterProvider: o.meterProvider,
	}

	meter := o.meterProvider.Meter(compositeInstrumentationName)
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

// Create stores v under its own composite key, indexed by indexOf(v).
// Returns ports.ErrConflict if a document with that key already exists in
// this collection.
func (r *CompositeRepository[T]) Create(ctx context.Context, v *T) error {
	id := r.id(v)
	ctx, span := r.tracer.Start(ctx, r.collection+"_repository.create",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String(r.collection+".key", id)))
	defer span.End()

	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("adapters/store: marshal %s %s: %w", r.collection, id, err)
	}

	if err := r.ds.Create(ctx, datastore.Document{Collection: r.collection, ID: id, Data: data, Index: r.indexOf(v)}); err != nil {
		return err
	}

	r.creates.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))
	r.logger.DebugContext(ctx, r.collection+" created", r.collection+".key", id)
	return nil
}

// CreateBatch stores every v in vs under its own composite key, each
// indexed by indexOf(v), as a single atomic Datastore transaction — all
// succeed or none do. Only entities with a real bulk-create API endpoint
// call this — see docs/adr/0016-bulk-operations.md.
func (r *CompositeRepository[T]) CreateBatch(ctx context.Context, vs []*T) error {
	ctx, span := r.tracer.Start(ctx, r.collection+"_repository.create_batch",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.Int(r.collection+".count", len(vs))))
	defer span.End()

	docs := make([]datastore.Document, 0, len(vs))
	for _, v := range vs {
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("adapters/store: marshal %s %s: %w", r.collection, r.id(v), err)
		}
		docs = append(docs, datastore.Document{Collection: r.collection, ID: r.id(v), Data: data, Index: r.indexOf(v)})
	}

	if err := r.ds.CreateBatch(ctx, docs); err != nil {
		return err
	}

	r.creates.Add(ctx, int64(len(vs)), metric.WithAttributes(attribute.String("repository.name", r.name)))
	r.logger.DebugContext(ctx, r.collection+" batch created", "count", len(vs))
	return nil
}

// Get returns the record stored under k1/k2/k3. Returns ports.ErrNotFound
// if none exists.
func (r *CompositeRepository[T]) Get(ctx context.Context, k1, k2, k3 string) (*T, error) {
	id := joinKey(k1, k2, k3)
	ctx, span := r.tracer.Start(ctx, r.collection+"_repository.get",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String(r.collection+".key", id)))
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

// Update replaces the record stored under v's own composite key,
// re-indexed by indexOf(v). Returns ports.ErrNotFound if none exists.
func (r *CompositeRepository[T]) Update(ctx context.Context, v *T) error {
	id := r.id(v)
	ctx, span := r.tracer.Start(ctx, r.collection+"_repository.update",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String(r.collection+".key", id)))
	defer span.End()

	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("adapters/store: marshal %s %s: %w", r.collection, id, err)
	}

	if err := r.ds.Update(ctx, datastore.Document{Collection: r.collection, ID: id, Data: data, Index: r.indexOf(v)}); err != nil {
		return err
	}

	r.updates.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))
	r.logger.DebugContext(ctx, r.collection+" updated", r.collection+".key", id)
	return nil
}

// Delete removes the record stored under k1/k2/k3. Returns
// ports.ErrNotFound if none exists.
func (r *CompositeRepository[T]) Delete(ctx context.Context, k1, k2, k3 string) error {
	id := joinKey(k1, k2, k3)
	ctx, span := r.tracer.Start(ctx, r.collection+"_repository.delete",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String(r.collection+".key", id)))
	defer span.End()

	if err := r.ds.Delete(ctx, r.collection, id); err != nil {
		return err
	}

	r.deletes.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))
	r.logger.DebugContext(ctx, r.collection+" deleted", r.collection+".key", id)
	return nil
}

// List returns records in this collection matching filter (a non-empty
// filter restricts results to documents whose Index matches every entry;
// nil/empty means unfiltered), cursor-paginated. Which keys filter accepts
// is up to the caller (each entity's thin wrapper), matching whatever
// indexOf declared.
func (r *CompositeRepository[T]) List(ctx context.Context, filter map[string]string, pageSize int, pageToken string) ([]*T, string, error) {
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

func (r *CompositeRepository[T]) id(v *T) string {
	k1, k2, k3 := r.keyOf(v)
	return joinKey(k1, k2, k3)
}

// DocumentID returns the opaque datastore.Document ID v's composite key
// joins into. A hand-written adapter that must write v's document directly
// via Datastore.CreateBatch alongside a document in a different collection
// (bypassing Create/CreateBatch above — see
// internal/adapters/store/externalid, and
// docs/adr/0026-external-id-get-or-create.md) uses this to construct that
// document with the exact same ID CompositeRepository's own methods would
// use, instead of re-deriving the \x00-join convention itself.
func (r *CompositeRepository[T]) DocumentID(v *T) string { return r.id(v) }

// IndexOf returns the Index map v's document is stored under, per this
// CompositeRepository[T]'s indexOf — see DocumentID.
func (r *CompositeRepository[T]) IndexOf(v *T) map[string]string { return r.indexOf(v) }

func joinKey(k1, k2, k3 string) string {
	return k1 + keySeparator + k2 + keySeparator + k3
}

// Logger returns the logger this CompositeRepository[T] was constructed
// with. For a hand-written adapter that embeds a CompositeRepository[T] for
// part of its shape (see internal/adapters/store/externalid) and adds its
// own instrumentation around it, reusing this avoids re-deriving options
// from the same opts list a second time with a different (and possibly
// inconsistent) result — see Repository[T].Logger.
func (r *CompositeRepository[T]) Logger() *slog.Logger { return r.logger }

// Tracer returns the tracer this CompositeRepository[T] was constructed
// with — see Logger.
func (r *CompositeRepository[T]) Tracer() trace.Tracer { return r.tracer }

// MeterProvider returns the MeterProvider this CompositeRepository[T] was
// constructed with — see Logger.
func (r *CompositeRepository[T]) MeterProvider() metric.MeterProvider { return r.meterProvider }
