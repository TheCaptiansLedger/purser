package sql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"purser/internal/adapters/datastore"
	"purser/internal/ports"
	"sort"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const instrumentationName = "purser/internal/adapters/datastore/sql"

// Store is the database/sql-backed datastore.Datastore implementation,
// shared by every dialect in Dialect — see
// docs/adr/0012-datastore-persistence.md.
type Store struct {
	db      *sql.DB
	dialect Dialect
	name    string

	logger *slog.Logger
	tracer trace.Tracer

	creates metric.Int64Counter
	gets    metric.Int64Counter
	updates metric.Int64Counter
	deletes metric.Int64Counter
	lists   metric.Int64Counter
}

var _ datastore.Datastore = (*Store)(nil)

// New constructs a named Store backed by db (see Open) for dialect.
func New(name string, db *sql.DB, dialect Dialect, opts ...Option) (*Store, error) {
	if name == "" {
		return nil, fmt.Errorf("datastore/sql: name must not be empty")
	}
	if db == nil {
		return nil, fmt.Errorf("datastore/sql: db must not be nil")
	}

	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	s := &Store{
		db:      db,
		dialect: dialect,
		name:    name,
		logger:  o.logger.With("component", "adapters.datastore.sql", "datastore.name", name, "datastore.dialect", string(dialect)),
		tracer:  o.tracerProvider.Tracer(instrumentationName),
	}

	meter := o.meterProvider.Meter(instrumentationName)
	var err error
	if s.creates, err = meter.Int64Counter("datastore_sql.creates", metric.WithDescription("Documents created")); err != nil {
		return nil, fmt.Errorf("datastore/sql: creating creates counter: %w", err)
	}
	if s.gets, err = meter.Int64Counter("datastore_sql.gets", metric.WithDescription("Get calls")); err != nil {
		return nil, fmt.Errorf("datastore/sql: creating gets counter: %w", err)
	}
	if s.updates, err = meter.Int64Counter("datastore_sql.updates", metric.WithDescription("Documents updated")); err != nil {
		return nil, fmt.Errorf("datastore/sql: creating updates counter: %w", err)
	}
	if s.deletes, err = meter.Int64Counter("datastore_sql.deletes", metric.WithDescription("Documents deleted")); err != nil {
		return nil, fmt.Errorf("datastore/sql: creating deletes counter: %w", err)
	}
	if s.lists, err = meter.Int64Counter("datastore_sql.lists", metric.WithDescription("List calls")); err != nil {
		return nil, fmt.Errorf("datastore/sql: creating lists counter: %w", err)
	}

	s.logger.Info("sql datastore created")
	return s, nil
}

// Create implements datastore.Datastore.
func (s *Store) Create(ctx context.Context, doc datastore.Document) error {
	ctx, span := s.tracer.Start(ctx, "datastore_sql.create", trace.WithAttributes(
		attribute.String("datastore.name", s.name),
		attribute.String("document.collection", doc.Collection),
		attribute.String("document.id", doc.ID),
	))
	defer span.End()

	indexJSON, err := marshalIndex(doc.Index)
	if err != nil {
		return fmt.Errorf("datastore/sql: create %s/%s: %w", doc.Collection, doc.ID, err)
	}

	err = withTx(ctx, s.db, func(tx *sql.Tx) error {
		insert := rebind(`INSERT INTO documents(collection, id, data, index_json) VALUES(?, ?, ?, ?)`, s.dialect)
		if _, err := tx.ExecContext(ctx, insert, doc.Collection, doc.ID, string(doc.Data), indexJSON); err != nil {
			if isConflict(s.dialect, err) {
				return ports.ErrConflict
			}
			return err
		}
		return insertIndexRows(ctx, tx, s.dialect, doc.Collection, doc.ID, doc.Index)
	})
	if err != nil {
		if errors.Is(err, ports.ErrConflict) {
			return err
		}
		return fmt.Errorf("datastore/sql: create %s/%s: %w", doc.Collection, doc.ID, err)
	}

	s.creates.Add(ctx, 1, metric.WithAttributes(attribute.String("datastore.name", s.name)))
	s.logger.DebugContext(ctx, "document created", "document.collection", doc.Collection, "document.id", doc.ID)
	return nil
}

// Get implements datastore.Datastore.
func (s *Store) Get(ctx context.Context, collection, id string) (datastore.Document, error) {
	ctx, span := s.tracer.Start(ctx, "datastore_sql.get", trace.WithAttributes(
		attribute.String("datastore.name", s.name),
		attribute.String("document.collection", collection),
		attribute.String("document.id", id),
	))
	defer span.End()

	s.gets.Add(ctx, 1, metric.WithAttributes(attribute.String("datastore.name", s.name)))

	query := rebind(`SELECT data, index_json FROM documents WHERE collection = ? AND id = ?`, s.dialect)
	var data, indexJSON string
	err := s.db.QueryRowContext(ctx, query, collection, id).Scan(&data, &indexJSON)
	if errors.Is(err, sql.ErrNoRows) {
		span.SetAttributes(attribute.Bool("document.found", false))
		return datastore.Document{}, ports.ErrNotFound
	}
	if err != nil {
		return datastore.Document{}, fmt.Errorf("datastore/sql: get %s/%s: %w", collection, id, err)
	}

	index, err := unmarshalIndex(indexJSON)
	if err != nil {
		return datastore.Document{}, fmt.Errorf("datastore/sql: get %s/%s: %w", collection, id, err)
	}
	return datastore.Document{Collection: collection, ID: id, Data: []byte(data), Index: index}, nil
}

// Update implements datastore.Datastore.
func (s *Store) Update(ctx context.Context, doc datastore.Document) error {
	ctx, span := s.tracer.Start(ctx, "datastore_sql.update", trace.WithAttributes(
		attribute.String("datastore.name", s.name),
		attribute.String("document.collection", doc.Collection),
		attribute.String("document.id", doc.ID),
	))
	defer span.End()

	indexJSON, err := marshalIndex(doc.Index)
	if err != nil {
		return fmt.Errorf("datastore/sql: update %s/%s: %w", doc.Collection, doc.ID, err)
	}

	err = withTx(ctx, s.db, func(tx *sql.Tx) error {
		update := rebind(`UPDATE documents SET data = ?, index_json = ? WHERE collection = ? AND id = ?`, s.dialect)
		res, err := tx.ExecContext(ctx, update, string(doc.Data), indexJSON, doc.Collection, doc.ID)
		if err != nil {
			return err
		}
		rows, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if rows == 0 {
			return ports.ErrNotFound
		}

		del := rebind(`DELETE FROM document_index WHERE collection = ? AND id = ?`, s.dialect)
		if _, err := tx.ExecContext(ctx, del, doc.Collection, doc.ID); err != nil {
			return err
		}
		return insertIndexRows(ctx, tx, s.dialect, doc.Collection, doc.ID, doc.Index)
	})
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			return err
		}
		return fmt.Errorf("datastore/sql: update %s/%s: %w", doc.Collection, doc.ID, err)
	}

	s.updates.Add(ctx, 1, metric.WithAttributes(attribute.String("datastore.name", s.name)))
	s.logger.DebugContext(ctx, "document updated", "document.collection", doc.Collection, "document.id", doc.ID)
	return nil
}

// Delete implements datastore.Datastore.
func (s *Store) Delete(ctx context.Context, collection, id string) error {
	ctx, span := s.tracer.Start(ctx, "datastore_sql.delete", trace.WithAttributes(
		attribute.String("datastore.name", s.name),
		attribute.String("document.collection", collection),
		attribute.String("document.id", id),
	))
	defer span.End()

	err := withTx(ctx, s.db, func(tx *sql.Tx) error {
		del := rebind(`DELETE FROM documents WHERE collection = ? AND id = ?`, s.dialect)
		res, err := tx.ExecContext(ctx, del, collection, id)
		if err != nil {
			return err
		}
		rows, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if rows == 0 {
			return ports.ErrNotFound
		}

		delIdx := rebind(`DELETE FROM document_index WHERE collection = ? AND id = ?`, s.dialect)
		_, err = tx.ExecContext(ctx, delIdx, collection, id)
		return err
	})
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			return err
		}
		return fmt.Errorf("datastore/sql: delete %s/%s: %w", collection, id, err)
	}

	s.deletes.Add(ctx, 1, metric.WithAttributes(attribute.String("datastore.name", s.name)))
	s.logger.DebugContext(ctx, "document deleted", "document.collection", collection, "document.id", id)
	return nil
}

// List implements datastore.Datastore. An empty filter is a
// PRIMARY KEY-ordered range scan (real index, no full scan); a non-empty
// filter does an indexed lookup per filter key against document_index and
// intersects the resulting ID sets before batch-fetching only the
// matching rows — see docs/adr/0012-datastore-persistence.md.
func (s *Store) List(ctx context.Context, collection string, filter map[string]string, pageSize int, pageToken string) ([]datastore.Document, string, error) {
	ctx, span := s.tracer.Start(ctx, "datastore_sql.list", trace.WithAttributes(
		attribute.String("datastore.name", s.name),
		attribute.String("document.collection", collection),
	))
	defer span.End()

	if pageSize <= 0 {
		pageSize = 50
	}

	// Fetch one extra row beyond pageSize so "more pages exist" can be
	// detected exactly, instead of guessing from a page that happens to
	// come back exactly pageSize long.
	var docs []datastore.Document
	var err error
	if len(filter) == 0 {
		docs, err = s.listUnfiltered(ctx, collection, pageToken, pageSize+1)
	} else {
		docs, err = s.listFiltered(ctx, collection, filter, pageToken, pageSize+1)
	}
	if err != nil {
		return nil, "", fmt.Errorf("datastore/sql: list %s: %w", collection, err)
	}

	var nextToken string
	if len(docs) > pageSize {
		docs = docs[:pageSize]
		nextToken = docs[len(docs)-1].ID
	}

	s.lists.Add(ctx, 1, metric.WithAttributes(attribute.String("datastore.name", s.name)))
	s.logger.DebugContext(ctx, "document list", "document.collection", collection, "count", len(docs), "next_page_token", nextToken)
	return docs, nextToken, nil
}

// CreateBatch implements datastore.Datastore.
func (s *Store) CreateBatch(ctx context.Context, docs []datastore.Document) error {
	ctx, span := s.tracer.Start(ctx, "datastore_sql.create_batch", trace.WithAttributes(
		attribute.String("datastore.name", s.name),
		attribute.Int("document.count", len(docs)),
	))
	defer span.End()

	err := withTx(ctx, s.db, func(tx *sql.Tx) error {
		insert := rebind(`INSERT INTO documents(collection, id, data, index_json) VALUES(?, ?, ?, ?)`, s.dialect)
		for _, d := range docs {
			indexJSON, err := marshalIndex(d.Index)
			if err != nil {
				return fmt.Errorf("marshal index for %s/%s: %w", d.Collection, d.ID, err)
			}
			if _, err := tx.ExecContext(ctx, insert, d.Collection, d.ID, string(d.Data), indexJSON); err != nil {
				if isConflict(s.dialect, err) {
					return ports.ErrConflict
				}
				return err
			}
			if err := insertIndexRows(ctx, tx, s.dialect, d.Collection, d.ID, d.Index); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, ports.ErrConflict) {
			return err
		}
		return fmt.Errorf("datastore/sql: create batch: %w", err)
	}

	s.creates.Add(ctx, int64(len(docs)), metric.WithAttributes(attribute.String("datastore.name", s.name)))
	s.logger.DebugContext(ctx, "document batch created", "document.count", len(docs))
	return nil
}

// DeleteBatch implements datastore.Datastore.
func (s *Store) DeleteBatch(ctx context.Context, collection string, ids []string) error {
	ctx, span := s.tracer.Start(ctx, "datastore_sql.delete_batch", trace.WithAttributes(
		attribute.String("datastore.name", s.name),
		attribute.String("document.collection", collection),
		attribute.Int("document.count", len(ids)),
	))
	defer span.End()

	err := withTx(ctx, s.db, func(tx *sql.Tx) error {
		del := rebind(`DELETE FROM documents WHERE collection = ? AND id = ?`, s.dialect)
		delIdx := rebind(`DELETE FROM document_index WHERE collection = ? AND id = ?`, s.dialect)
		for _, id := range ids {
			res, err := tx.ExecContext(ctx, del, collection, id)
			if err != nil {
				return err
			}
			rows, err := res.RowsAffected()
			if err != nil {
				return err
			}
			if rows == 0 {
				return ports.ErrNotFound
			}
			if _, err := tx.ExecContext(ctx, delIdx, collection, id); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			return err
		}
		return fmt.Errorf("datastore/sql: delete batch %s: %w", collection, err)
	}

	s.deletes.Add(ctx, int64(len(ids)), metric.WithAttributes(attribute.String("datastore.name", s.name)))
	s.logger.DebugContext(ctx, "document batch deleted", "document.collection", collection, "document.count", len(ids))
	return nil
}

func (s *Store) listUnfiltered(ctx context.Context, collection, pageToken string, limit int) ([]datastore.Document, error) {
	query := rebind(`SELECT id, data, index_json FROM documents WHERE collection = ? AND id > ? ORDER BY id LIMIT ?`, s.dialect)
	rows, err := s.db.QueryContext(ctx, query, collection, pageToken, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	docs := make([]datastore.Document, 0, limit)
	for rows.Next() {
		var id, data, indexJSON string
		if err := rows.Scan(&id, &data, &indexJSON); err != nil {
			return nil, err
		}
		index, err := unmarshalIndex(indexJSON)
		if err != nil {
			return nil, err
		}
		docs = append(docs, datastore.Document{Collection: collection, ID: id, Data: []byte(data), Index: index})
	}
	return docs, rows.Err()
}

func (s *Store) listFiltered(ctx context.Context, collection string, filter map[string]string, pageToken string, limit int) ([]datastore.Document, error) {
	matched, err := s.filteredIDs(ctx, collection, filter)
	if err != nil {
		return nil, err
	}

	ids := paginateIDs(matched, pageToken, limit)
	if len(ids) == 0 {
		return []datastore.Document{}, nil
	}

	return s.fetchDocuments(ctx, collection, ids)
}

// filteredIDs returns the set of document IDs matching every key/value
// pair in filter (AND semantics), via one indexed document_index lookup
// per filter key intersected in Go.
func (s *Store) filteredIDs(ctx context.Context, collection string, filter map[string]string) (map[string]struct{}, error) {
	var matched map[string]struct{}
	for key, value := range filter {
		set, err := s.indexLookup(ctx, collection, key, value)
		if err != nil {
			return nil, err
		}
		if matched == nil {
			matched = set
			continue
		}
		matched = intersect(matched, set)
	}
	return matched, nil
}

func (s *Store) indexLookup(ctx context.Context, collection, key, value string) (map[string]struct{}, error) {
	query := rebind(`SELECT id FROM document_index WHERE collection = ? AND index_key = ? AND index_value = ?`, s.dialect)
	rows, err := s.db.QueryContext(ctx, query, collection, key, value)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	set := make(map[string]struct{})
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		set[id] = struct{}{}
	}
	return set, rows.Err()
}

// paginateIDs sorts matched, drops anything at or before pageToken, and
// truncates to limit.
func paginateIDs(matched map[string]struct{}, pageToken string, limit int) []string {
	ids := make([]string, 0, len(matched))
	for id := range matched {
		if id > pageToken {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	if len(ids) > limit {
		ids = ids[:limit]
	}
	return ids
}

// fetchDocuments batch-fetches ids from documents and returns them in the
// same order ids was given in.
func (s *Store) fetchDocuments(ctx context.Context, collection string, ids []string) ([]datastore.Document, error) {
	placeholders := make([]string, len(ids))
	args := make([]any, 0, len(ids)+1)
	args = append(args, collection)
	for i, id := range ids {
		placeholders[i] = "?"
		args = append(args, id)
	}
	query := rebind(fmt.Sprintf(`SELECT id, data, index_json FROM documents WHERE collection = ? AND id IN (%s)`, strings.Join(placeholders, ", ")), s.dialect)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	byID := make(map[string]datastore.Document, len(ids))
	for rows.Next() {
		var id, data, indexJSON string
		if err := rows.Scan(&id, &data, &indexJSON); err != nil {
			return nil, err
		}
		index, err := unmarshalIndex(indexJSON)
		if err != nil {
			return nil, err
		}
		byID[id] = datastore.Document{Collection: collection, ID: id, Data: []byte(data), Index: index}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	docs := make([]datastore.Document, 0, len(ids))
	for _, id := range ids {
		docs = append(docs, byID[id])
	}
	return docs, nil
}

func withTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func insertIndexRows(ctx context.Context, tx *sql.Tx, dialect Dialect, collection, id string, index map[string]string) error {
	if len(index) == 0 {
		return nil
	}
	insert := rebind(`INSERT INTO document_index(collection, index_key, index_value, id) VALUES(?, ?, ?, ?)`, dialect)
	for key, value := range index {
		if _, err := tx.ExecContext(ctx, insert, collection, key, value, id); err != nil {
			return err
		}
	}
	return nil
}

func marshalIndex(index map[string]string) (string, error) {
	if len(index) == 0 {
		return "{}", nil
	}
	b, err := json.Marshal(index)
	if err != nil {
		return "", fmt.Errorf("marshal index: %w", err)
	}
	return string(b), nil
}

func unmarshalIndex(indexJSON string) (map[string]string, error) {
	if indexJSON == "" || indexJSON == "{}" {
		return map[string]string{}, nil
	}
	var index map[string]string
	if err := json.Unmarshal([]byte(indexJSON), &index); err != nil {
		return nil, fmt.Errorf("unmarshal index: %w", err)
	}
	return index, nil
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
