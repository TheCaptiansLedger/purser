package badger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"purser/internal/adapters/datastore"
	"purser/internal/ports"
	"sort"

	badgerdb "github.com/dgraph-io/badger/v4"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const instrumentationName = "purser/internal/adapters/datastore/badger"

// envelope is the on-disk shape of a datastore.Document's primary record.
// Data is kept as json.RawMessage so the caller's already-JSON-encoded
// entity isn't re-escaped into a string.
type envelope struct {
	Data  json.RawMessage   `json:"data"`
	Index map[string]string `json:"index,omitempty"`
}

// Store is the Badger-backed datastore.Datastore implementation. Safe for
// concurrent use — BadgerDB serializes writes via its own MVCC
// transactions.
type Store struct {
	db   *badgerdb.DB
	name string

	logger *slog.Logger
	tracer trace.Tracer

	creates metric.Int64Counter
	gets    metric.Int64Counter
	updates metric.Int64Counter
	deletes metric.Int64Counter
	lists   metric.Int64Counter
}

var _ datastore.Datastore = (*Store)(nil)

// New constructs a named Store backed by db (see Open).
func New(name string, db *badgerdb.DB, opts ...Option) (*Store, error) {
	if name == "" {
		return nil, fmt.Errorf("datastore/badger: name must not be empty")
	}
	if db == nil {
		return nil, fmt.Errorf("datastore/badger: db must not be nil")
	}

	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	s := &Store{
		db:     db,
		name:   name,
		logger: o.logger.With("component", "adapters.datastore.badger", "datastore.name", name),
		tracer: o.tracerProvider.Tracer(instrumentationName),
	}

	meter := o.meterProvider.Meter(instrumentationName)
	var err error
	if s.creates, err = meter.Int64Counter("datastore_badger.creates", metric.WithDescription("Documents created")); err != nil {
		return nil, fmt.Errorf("datastore/badger: creating creates counter: %w", err)
	}
	if s.gets, err = meter.Int64Counter("datastore_badger.gets", metric.WithDescription("Get calls")); err != nil {
		return nil, fmt.Errorf("datastore/badger: creating gets counter: %w", err)
	}
	if s.updates, err = meter.Int64Counter("datastore_badger.updates", metric.WithDescription("Documents updated")); err != nil {
		return nil, fmt.Errorf("datastore/badger: creating updates counter: %w", err)
	}
	if s.deletes, err = meter.Int64Counter("datastore_badger.deletes", metric.WithDescription("Documents deleted")); err != nil {
		return nil, fmt.Errorf("datastore/badger: creating deletes counter: %w", err)
	}
	if s.lists, err = meter.Int64Counter("datastore_badger.lists", metric.WithDescription("List calls")); err != nil {
		return nil, fmt.Errorf("datastore/badger: creating lists counter: %w", err)
	}

	s.logger.Info("badger datastore created")
	return s, nil
}

// Create implements datastore.Datastore.
func (s *Store) Create(ctx context.Context, doc datastore.Document) error {
	ctx, span := s.tracer.Start(ctx, "datastore_badger.create", trace.WithAttributes(
		attribute.String("datastore.name", s.name),
		attribute.String("document.collection", doc.Collection),
		attribute.String("document.id", doc.ID),
	))
	defer span.End()

	err := s.db.Update(func(txn *badgerdb.Txn) error {
		_, getErr := txn.Get(kPrimary(doc.Collection, doc.ID))
		if getErr == nil {
			return ports.ErrConflict
		}
		if !errors.Is(getErr, badgerdb.ErrKeyNotFound) {
			return getErr
		}
		return writeDoc(txn, doc)
	})
	if err != nil {
		if errors.Is(err, ports.ErrConflict) {
			return err
		}
		return fmt.Errorf("datastore/badger: create %s/%s: %w", doc.Collection, doc.ID, err)
	}

	s.creates.Add(ctx, 1, metric.WithAttributes(attribute.String("datastore.name", s.name)))
	s.logger.DebugContext(ctx, "document created", "document.collection", doc.Collection, "document.id", doc.ID)
	return nil
}

// Get implements datastore.Datastore.
func (s *Store) Get(ctx context.Context, collection, id string) (datastore.Document, error) {
	ctx, span := s.tracer.Start(ctx, "datastore_badger.get", trace.WithAttributes(
		attribute.String("datastore.name", s.name),
		attribute.String("document.collection", collection),
		attribute.String("document.id", id),
	))
	defer span.End()

	s.gets.Add(ctx, 1, metric.WithAttributes(attribute.String("datastore.name", s.name)))

	var doc datastore.Document
	err := s.db.View(func(txn *badgerdb.Txn) error {
		env, loadErr := loadEnvelope(txn, collection, id)
		if loadErr != nil {
			return loadErr
		}
		doc = datastore.Document{Collection: collection, ID: id, Data: []byte(env.Data), Index: env.Index}
		return nil
	})
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			span.SetAttributes(attribute.Bool("document.found", false))
			return datastore.Document{}, err
		}
		return datastore.Document{}, fmt.Errorf("datastore/badger: get %s/%s: %w", collection, id, err)
	}
	return doc, nil
}

// Update implements datastore.Datastore.
func (s *Store) Update(ctx context.Context, doc datastore.Document) error {
	ctx, span := s.tracer.Start(ctx, "datastore_badger.update", trace.WithAttributes(
		attribute.String("datastore.name", s.name),
		attribute.String("document.collection", doc.Collection),
		attribute.String("document.id", doc.ID),
	))
	defer span.End()

	err := s.db.Update(func(txn *badgerdb.Txn) error {
		old, loadErr := loadEnvelope(txn, doc.Collection, doc.ID)
		if loadErr != nil {
			return loadErr
		}
		if delErr := deleteIndexEntries(txn, doc.Collection, doc.ID, old.Index); delErr != nil {
			return delErr
		}
		return writeDoc(txn, doc)
	})
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			return err
		}
		return fmt.Errorf("datastore/badger: update %s/%s: %w", doc.Collection, doc.ID, err)
	}

	s.updates.Add(ctx, 1, metric.WithAttributes(attribute.String("datastore.name", s.name)))
	s.logger.DebugContext(ctx, "document updated", "document.collection", doc.Collection, "document.id", doc.ID)
	return nil
}

// Delete implements datastore.Datastore.
func (s *Store) Delete(ctx context.Context, collection, id string) error {
	ctx, span := s.tracer.Start(ctx, "datastore_badger.delete", trace.WithAttributes(
		attribute.String("datastore.name", s.name),
		attribute.String("document.collection", collection),
		attribute.String("document.id", id),
	))
	defer span.End()

	err := s.db.Update(func(txn *badgerdb.Txn) error {
		old, loadErr := loadEnvelope(txn, collection, id)
		if loadErr != nil {
			return loadErr
		}
		if delErr := deleteIndexEntries(txn, collection, id, old.Index); delErr != nil {
			return delErr
		}
		return txn.Delete(kPrimary(collection, id))
	})
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			return err
		}
		return fmt.Errorf("datastore/badger: delete %s/%s: %w", collection, id, err)
	}

	s.deletes.Add(ctx, 1, metric.WithAttributes(attribute.String("datastore.name", s.name)))
	s.logger.DebugContext(ctx, "document deleted", "document.collection", collection, "document.id", id)
	return nil
}

// List implements datastore.Datastore. An empty filter prefix-scans the
// whole collection (already ID-ordered); a non-empty filter intersects
// per-key secondary-index prefix scans instead of scanning every document
// in the collection — see docs/adr/0012-datastore-persistence.md.
func (s *Store) List(ctx context.Context, collection string, filter map[string]string, pageSize int, pageToken string) ([]datastore.Document, string, error) {
	ctx, span := s.tracer.Start(ctx, "datastore_badger.list", trace.WithAttributes(
		attribute.String("datastore.name", s.name),
		attribute.String("document.collection", collection),
	))
	defer span.End()

	if pageSize <= 0 {
		pageSize = 50
	}

	var ids []string
	err := s.db.View(func(txn *badgerdb.Txn) error {
		var idErr error
		ids, idErr = matchingIDs(txn, collection, filter)
		return idErr
	})
	if err != nil {
		return nil, "", fmt.Errorf("datastore/badger: list %s: %w", collection, err)
	}
	sort.Strings(ids)

	start := 0
	if pageToken != "" {
		start = sort.SearchStrings(ids, pageToken)
		if start < len(ids) && ids[start] == pageToken {
			start++
		}
	}
	end := min(start+pageSize, len(ids))

	docs := make([]datastore.Document, 0, end-start)
	err = s.db.View(func(txn *badgerdb.Txn) error {
		for _, id := range ids[start:end] {
			env, loadErr := loadEnvelope(txn, collection, id)
			if loadErr != nil {
				return loadErr
			}
			docs = append(docs, datastore.Document{Collection: collection, ID: id, Data: []byte(env.Data), Index: env.Index})
		}
		return nil
	})
	if err != nil {
		return nil, "", fmt.Errorf("datastore/badger: list %s: %w", collection, err)
	}

	var nextToken string
	if end < len(ids) {
		nextToken = ids[end-1]
	}

	s.lists.Add(ctx, 1, metric.WithAttributes(attribute.String("datastore.name", s.name)))
	s.logger.DebugContext(ctx, "document list", "document.collection", collection, "count", len(docs), "next_page_token", nextToken)
	return docs, nextToken, nil
}

// CreateBatch implements datastore.Datastore.
func (s *Store) CreateBatch(ctx context.Context, docs []datastore.Document) error {
	ctx, span := s.tracer.Start(ctx, "datastore_badger.create_batch", trace.WithAttributes(
		attribute.String("datastore.name", s.name),
		attribute.Int("document.count", len(docs)),
	))
	defer span.End()

	err := s.db.Update(func(txn *badgerdb.Txn) error {
		for _, doc := range docs {
			_, getErr := txn.Get(kPrimary(doc.Collection, doc.ID))
			if getErr == nil {
				return ports.ErrConflict
			}
			if !errors.Is(getErr, badgerdb.ErrKeyNotFound) {
				return getErr
			}
			if err := writeDoc(txn, doc); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, ports.ErrConflict) {
			return err
		}
		return fmt.Errorf("datastore/badger: create batch: %w", err)
	}

	s.creates.Add(ctx, int64(len(docs)), metric.WithAttributes(attribute.String("datastore.name", s.name)))
	s.logger.DebugContext(ctx, "document batch created", "document.count", len(docs))
	return nil
}

// DeleteBatch implements datastore.Datastore.
func (s *Store) DeleteBatch(ctx context.Context, collection string, ids []string) error {
	ctx, span := s.tracer.Start(ctx, "datastore_badger.delete_batch", trace.WithAttributes(
		attribute.String("datastore.name", s.name),
		attribute.String("document.collection", collection),
		attribute.Int("document.count", len(ids)),
	))
	defer span.End()

	err := s.db.Update(func(txn *badgerdb.Txn) error {
		for _, id := range ids {
			old, loadErr := loadEnvelope(txn, collection, id)
			if loadErr != nil {
				return loadErr
			}
			if delErr := deleteIndexEntries(txn, collection, id, old.Index); delErr != nil {
				return delErr
			}
			if err := txn.Delete(kPrimary(collection, id)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			return err
		}
		return fmt.Errorf("datastore/badger: delete batch %s: %w", collection, err)
	}

	s.deletes.Add(ctx, int64(len(ids)), metric.WithAttributes(attribute.String("datastore.name", s.name)))
	s.logger.DebugContext(ctx, "document batch deleted", "document.collection", collection, "document.count", len(ids))
	return nil
}

// UpdateBatch implements datastore.Datastore.
func (s *Store) UpdateBatch(ctx context.Context, docs []datastore.Document) error {
	ctx, span := s.tracer.Start(ctx, "datastore_badger.update_batch", trace.WithAttributes(
		attribute.String("datastore.name", s.name),
		attribute.Int("document.count", len(docs)),
	))
	defer span.End()

	err := s.db.Update(func(txn *badgerdb.Txn) error {
		for _, doc := range docs {
			old, loadErr := loadEnvelope(txn, doc.Collection, doc.ID)
			if loadErr != nil {
				return loadErr
			}
			if delErr := deleteIndexEntries(txn, doc.Collection, doc.ID, old.Index); delErr != nil {
				return delErr
			}
			if err := writeDoc(txn, doc); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			return err
		}
		return fmt.Errorf("datastore/badger: update batch: %w", err)
	}

	s.updates.Add(ctx, int64(len(docs)), metric.WithAttributes(attribute.String("datastore.name", s.name)))
	s.logger.DebugContext(ctx, "document batch updated", "document.count", len(docs))
	return nil
}

func writeDoc(txn *badgerdb.Txn, d datastore.Document) error {
	env := envelope{Data: json.RawMessage(d.Data), Index: d.Index}
	b, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal document: %w", err)
	}
	if err := txn.Set(kPrimary(d.Collection, d.ID), b); err != nil {
		return err
	}
	for k, v := range d.Index {
		if err := txn.Set(kIndex(d.Collection, k, v, d.ID), []byte{}); err != nil {
			return err
		}
	}
	return nil
}

func deleteIndexEntries(txn *badgerdb.Txn, collection, id string, index map[string]string) error {
	for k, v := range index {
		if err := txn.Delete(kIndex(collection, k, v, id)); err != nil {
			return err
		}
	}
	return nil
}

func loadEnvelope(txn *badgerdb.Txn, collection, id string) (envelope, error) {
	item, err := txn.Get(kPrimary(collection, id))
	if errors.Is(err, badgerdb.ErrKeyNotFound) {
		return envelope{}, ports.ErrNotFound
	}
	if err != nil {
		return envelope{}, err
	}
	var env envelope
	if err := item.Value(func(val []byte) error {
		return json.Unmarshal(val, &env)
	}); err != nil {
		return envelope{}, fmt.Errorf("unmarshal document: %w", err)
	}
	return env, nil
}

// matchingIDs returns every document ID in collection matching filter (AND
// semantics across keys), or every ID in collection if filter is empty.
func matchingIDs(txn *badgerdb.Txn, collection string, filter map[string]string) ([]string, error) {
	if len(filter) == 0 {
		prefix := pfxPrimary(collection)
		opts := badgerdb.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()

		var ids []string
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			key := it.Item().KeyCopy(nil)
			ids = append(ids, string(key[len(prefix):]))
		}
		return ids, nil
	}

	var sets []map[string]struct{}
	for key, value := range filter {
		prefix := pfxIndex(collection, key, value)
		opts := badgerdb.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)

		set := make(map[string]struct{})
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			k := it.Item().KeyCopy(nil)
			set[string(k[len(prefix):])] = struct{}{}
		}
		it.Close()
		sets = append(sets, set)
	}

	matched := sets[0]
	for _, set := range sets[1:] {
		matched = intersect(matched, set)
	}

	ids := make([]string, 0, len(matched))
	for id := range matched {
		ids = append(ids, id)
	}
	return ids, nil
}

func intersect(a, b map[string]struct{}) map[string]struct{} {
	out := make(map[string]struct{}, min(len(a), len(b)))
	for id := range a {
		if _, ok := b[id]; ok {
			out[id] = struct{}{}
		}
	}
	return out
}

// Option customizes a Store constructed via New.
type Option func(*options)

type options struct {
	logger         *slog.Logger
	tracerProvider trace.TracerProvider
	meterProvider  metric.MeterProvider
}

func defaultOptions() *options {
	return &options{
		logger:         slog.Default(),
		tracerProvider: otel.GetTracerProvider(),
		meterProvider:  otel.GetMeterProvider(),
	}
}

// WithLogger overrides the default (slog.Default()) logger.
func WithLogger(l *slog.Logger) Option { return func(o *options) { o.logger = l } }

// WithTracerProvider overrides the default (global) TracerProvider.
func WithTracerProvider(tp trace.TracerProvider) Option {
	return func(o *options) { o.tracerProvider = tp }
}

// WithMeterProvider overrides the default (global) MeterProvider.
func WithMeterProvider(mp metric.MeterProvider) Option {
	return func(o *options) { o.meterProvider = mp }
}
