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
// entity keyed on 3 joined string parts, filtering List by the first 2 —
// EntryPerson, ItemPerson, and (via a thin type-converting wrapper)
// ExternalID. See docs/adr/0012-datastore-persistence.md.
type CompositeRepository[T any] struct {
	name       string
	collection string
	ds         datastore.Datastore
	keyOf      func(*T) (string, string, string)

	logger *slog.Logger
	tracer trace.Tracer

	creates metric.Int64Counter
	gets    metric.Int64Counter
	updates metric.Int64Counter
	deletes metric.Int64Counter
	lists   metric.Int64Counter
}

// NewComposite constructs a named CompositeRepository[T] backed by ds,
// storing documents under collection. keyOf extracts T's 3 key parts —
// injected rather than required via a method on T, so this package makes
// no assumption about a domain type's field names.
func NewComposite[T any](name, collection string, ds datastore.Datastore, keyOf func(*T) (string, string, string), opts ...Option) (*CompositeRepository[T], error) {
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

	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	r := &CompositeRepository[T]{
		name:       name,
		collection: collection,
		ds:         ds,
		keyOf:      keyOf,
		logger:     o.logger.With("component", "adapters.store."+collection, "repository.name", name),
		tracer:     o.tracerProvider.Tracer(compositeInstrumentationName),
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

// Create stores v under its own composite key. Returns ports.ErrConflict
// if a document with that key already exists in this collection.
func (r *CompositeRepository[T]) Create(ctx context.Context, v *T) error {
	id := r.id(v)
	ctx, span := r.tracer.Start(ctx, r.collection+"_repository.create",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String(r.collection+".key", id)))
	defer span.End()

	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("adapters/store: marshal %s %s: %w", r.collection, id, err)
	}

	if err := r.ds.Create(ctx, datastore.Document{Collection: r.collection, ID: id, Data: data, Index: r.index(v)}); err != nil {
		return err
	}

	r.creates.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))
	r.logger.DebugContext(ctx, r.collection+" created", r.collection+".key", id)
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

// Update replaces the record stored under v's own composite key. Returns
// ports.ErrNotFound if none exists.
func (r *CompositeRepository[T]) Update(ctx context.Context, v *T) error {
	id := r.id(v)
	ctx, span := r.tracer.Start(ctx, r.collection+"_repository.update",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String(r.collection+".key", id)))
	defer span.End()

	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("adapters/store: marshal %s %s: %w", r.collection, id, err)
	}

	if err := r.ds.Update(ctx, datastore.Document{Collection: r.collection, ID: id, Data: data, Index: r.index(v)}); err != nil {
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

// List returns records in this collection, cursor-paginated. k1 and k2
// are independent, optional filters — an empty string means "no filter
// on this field," matching the in-memory adapters' documented behavior.
func (r *CompositeRepository[T]) List(ctx context.Context, k1, k2 string, pageSize int, pageToken string) ([]*T, string, error) {
	ctx, span := r.tracer.Start(ctx, r.collection+"_repository.list",
		trace.WithAttributes(attribute.String("repository.name", r.name)))
	defer span.End()

	filter := map[string]string{}
	if k1 != "" {
		filter["k1"] = k1
	}
	if k2 != "" {
		filter["k2"] = k2
	}
	if len(filter) == 0 {
		filter = nil
	}

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

func (r *CompositeRepository[T]) index(v *T) map[string]string {
	k1, k2, _ := r.keyOf(v)
	return map[string]string{"k1": k1, "k2": k2}
}

func joinKey(k1, k2, k3 string) string {
	return k1 + keySeparator + k2 + keySeparator + k3
}
