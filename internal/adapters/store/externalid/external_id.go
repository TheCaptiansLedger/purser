// Package externalid is the datastore-backed adapter for the
// ports.ExternalIDRepository port. Like internal/adapters/store/tag, this
// is a hand-written translator, not a thin CompositeRepository[T] wrapper —
// Create must additionally enforce get-or-create uniqueness on
// (EntityType, Source, Value), a shape CompositeRepository[T] alone doesn't
// cover. See docs/adr/0026-external-id-get-or-create.md for the full design
// and docs/adr/0019-tag-identity-and-get-or-create.md for the mechanism
// this reuses.
package externalid

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"purser/internal/adapters/datastore"
	"purser/internal/adapters/store"
	"purser/internal/domain"
	"purser/internal/ports"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const (
	collection = "external_id"

	// reservationCollection holds one document per unique (EntityType,
	// Source, Value) identity, pointing at the entity that currently owns
	// it — see docs/adr/0026-external-id-get-or-create.md.
	reservationCollection = "external_id_value"

	// keySeparator matches the composite-key convention
	// store.CompositeRepository[T] already uses.
	keySeparator = "\x00"

	instrumentationName = "purser/internal/adapters/store/externalid"
)

// Option customizes a Repository constructed via New.
type Option = store.Option

// WithLogger overrides the default (slog.Default()) logger.
var WithLogger = store.WithLogger

// WithTracerProvider overrides the default (global) TracerProvider.
var WithTracerProvider = store.WithTracerProvider

// WithMeterProvider overrides the default (global) MeterProvider.
var WithMeterProvider = store.WithMeterProvider

// reservation is the payload stored in a reservationCollection document.
type reservation struct {
	EntityID string `json:"entity_id"`
}

// Repository is the datastore-backed ports.ExternalIDRepository adapter.
// See docs/adr/0026-external-id-get-or-create.md.
type Repository struct {
	inner *store.CompositeRepository[domain.ExternalID]
	ds    datastore.Datastore

	logger *slog.Logger
	tracer trace.Tracer

	creates metric.Int64Counter
}

var _ ports.ExternalIDRepository = (*Repository)(nil)

// New constructs a named ports.ExternalIDRepository backed by ds.
func New(name string, ds datastore.Datastore, opts ...Option) (*Repository, error) {
	inner, err := store.NewComposite(name, collection, ds, keyOf, indexOf, opts...)
	if err != nil {
		return nil, err
	}

	r := &Repository{
		inner:  inner,
		ds:     ds,
		logger: inner.Logger().With("component", "adapters.store."+reservationCollection),
		tracer: inner.Tracer(),
	}

	meter := inner.MeterProvider().Meter(instrumentationName)
	if r.creates, err = meter.Int64Counter(collection+"_repository.creates", metric.WithDescription(collection+" records created")); err != nil {
		return nil, fmt.Errorf("adapters/store/externalid: creating creates counter: %w", err)
	}

	return r, nil
}

func keyOf(e *domain.ExternalID) (string, string, string) {
	return string(e.EntityType), e.EntityID, string(e.Source)
}

func indexOf(e *domain.ExternalID) map[string]string {
	return map[string]string{"k1": string(e.EntityType), "k2": e.EntityID}
}

func reservationID(entityType domain.EntityType, source domain.ExternalIDSource, value string) string {
	return string(entityType) + keySeparator + string(source) + keySeparator + value
}

// Get implements ports.ExternalIDRepository.
func (r *Repository) Get(ctx context.Context, entityType domain.EntityType, entityID, source string) (*domain.ExternalID, error) {
	return r.inner.Get(ctx, string(entityType), entityID, source)
}

// List implements ports.ExternalIDRepository.
func (r *Repository) List(ctx context.Context, entityType domain.EntityType, entityID string, pageSize int, pageToken string) ([]*domain.ExternalID, string, error) {
	filter := map[string]string{}
	if entityType != "" {
		filter["k1"] = string(entityType)
	}
	if entityID != "" {
		filter["k2"] = entityID
	}
	if len(filter) == 0 {
		filter = nil
	}
	return r.inner.List(ctx, filter, pageSize, pageToken)
}

// GetByValue implements ports.ExternalIDRepository, returning the row
// currently linking source/value to an entity of entityType, or
// ports.ErrNotFound if none does.
func (r *Repository) GetByValue(ctx context.Context, entityType domain.EntityType, source domain.ExternalIDSource, value string) (*domain.ExternalID, error) {
	ctx, span := r.tracer.Start(ctx, "external_id_repository.get_by_value", trace.WithAttributes(
		attribute.String("external_id.entity_type", string(entityType)), attribute.String("external_id.source", string(source)),
	))
	defer span.End()

	existing, found, err := r.resolveIdentity(ctx, entityType, source, value)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, ports.ErrNotFound
	}
	return existing, nil
}

// Create implements ports.ExternalIDRepository as get-or-create on
// (e.EntityType, e.Source, e.Value) — see
// docs/adr/0026-external-id-get-or-create.md. If a row already links that
// source/value to an entity, e is mutated in place to the pre-existing row
// (its incoming EntityID is discarded) and Create returns nil; a genuinely
// new identity is created as given. Returns ports.ErrConflict if e's own
// (EntityType, EntityID, Source) storage key is already taken by a
// different Value — use Update instead.
func (r *Repository) Create(ctx context.Context, e *domain.ExternalID) error {
	ctx, span := r.tracer.Start(ctx, "external_id_repository.create", trace.WithAttributes(
		attribute.String("external_id.entity_type", string(e.EntityType)),
		attribute.String("external_id.entity_id", e.EntityID),
		attribute.String("external_id.source", string(e.Source)),
	))
	defer span.End()

	if err := r.createOnce(ctx, e); err == nil {
		r.creates.Add(ctx, 1)
		r.logger.DebugContext(ctx, "external id created", "external_id.entity_id", e.EntityID)
		return nil
	} else if !errors.Is(err, ports.ErrConflict) {
		return err
	}

	existing, found, err := r.resolveIdentity(ctx, e.EntityType, e.Source, e.Value)
	if err != nil {
		return err
	}
	if found {
		span.SetAttributes(attribute.Bool("external_id.get_or_create_hit", true))
		*e = *existing
		return nil
	}

	// The reservation wasn't the cause of the conflict (or was stale and
	// has now been cleared) — retry once; a second conflict means the
	// caller's own (EntityType, EntityID, Source) collides with a
	// different Value already stored there.
	if err := r.createOnce(ctx, e); err != nil {
		return err
	}
	r.creates.Add(ctx, 1)
	r.logger.DebugContext(ctx, "external id created", "external_id.entity_id", e.EntityID)
	return nil
}

// createOnce atomically writes e's reservation and ExternalID documents via
// CreateBatch — the race-safe, constraint-based conflict check
// docs/adr/0012-datastore-persistence.md establishes, not a
// read-then-insert.
func (r *Repository) createOnce(ctx context.Context, e *domain.ExternalID) error {
	data, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("adapters/store/externalid: marshal external id %s: %w", e.EntityID, err)
	}
	resData, err := json.Marshal(reservation{EntityID: e.EntityID})
	if err != nil {
		return fmt.Errorf("adapters/store/externalid: marshal reservation for external id %s: %w", e.EntityID, err)
	}

	return r.ds.CreateBatch(ctx, []datastore.Document{
		{Collection: reservationCollection, ID: reservationID(e.EntityType, e.Source, e.Value), Data: resData},
		{Collection: collection, ID: r.inner.DocumentID(e), Data: data, Index: r.inner.IndexOf(e)},
	})
}

// resolveIdentity looks up the reservation for (entityType, source, value).
// If it exists and the row it points at still has a matching Value, that
// row is returned. If the reservation is missing, or points at a row that
// no longer exists or whose Value no longer matches (a stale reservation
// left by a crash mid-update or mid-delete — see
// docs/adr/0026-external-id-get-or-create.md), any stale reservation is
// deleted and (nil, false, nil) is returned so the caller can proceed as if
// no reservation existed.
func (r *Repository) resolveIdentity(ctx context.Context, entityType domain.EntityType, source domain.ExternalIDSource, value string) (*domain.ExternalID, bool, error) {
	id := reservationID(entityType, source, value)
	doc, err := r.ds.Get(ctx, reservationCollection, id)
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}

	var res reservation
	if err := json.Unmarshal(doc.Data, &res); err != nil {
		return nil, false, fmt.Errorf("adapters/store/externalid: unmarshal reservation %s: %w", id, err)
	}

	existing, err := r.inner.Get(ctx, string(entityType), res.EntityID, string(source))
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			r.deleteStaleReservation(ctx, id)
			return nil, false, nil
		}
		return nil, false, err
	}

	if existing.Value != value {
		r.deleteStaleReservation(ctx, id)
		return nil, false, nil
	}

	return existing, true, nil
}

func (r *Repository) deleteStaleReservation(ctx context.Context, id string) {
	if err := r.ds.Delete(ctx, reservationCollection, id); err != nil && !errors.Is(err, ports.ErrNotFound) {
		r.logger.WarnContext(ctx, "externalid: failed to clean up a stale/orphaned reservation", "reservation.id", id, "error", err)
	}
}

// Update implements ports.ExternalIDRepository. If e.Value differs from the
// existing row's Value, the (EntityType, Source, Value) reservation is
// moved under the same uniqueness constraint Create enforces, returning
// ports.ErrConflict if the new value is already owned by a different, live
// row — see docs/adr/0026-external-id-get-or-create.md's "Update may still
// change Value" section. If Value is unchanged, this delegates straight to
// the embedded translator.
func (r *Repository) Update(ctx context.Context, e *domain.ExternalID) error {
	ctx, span := r.tracer.Start(ctx, "external_id_repository.update", trace.WithAttributes(attribute.String("external_id.entity_id", e.EntityID)))
	defer span.End()

	existing, err := r.inner.Get(ctx, string(e.EntityType), e.EntityID, string(e.Source))
	if err != nil {
		return err
	}

	if existing.Value == e.Value {
		return r.inner.Update(ctx, e)
	}

	oldID := reservationID(existing.EntityType, existing.Source, existing.Value)
	newID := reservationID(e.EntityType, e.Source, e.Value)

	if err := r.reserve(ctx, newID, e); err != nil {
		return err
	}

	if err := r.inner.Update(ctx, e); err != nil {
		return err
	}

	// Best-effort: a crash here leaves oldID orphaned, blocking reuse of
	// the freed identity until the next Create/Update against it
	// self-heals via resolveIdentity. See
	// docs/adr/0026-external-id-get-or-create.md.
	r.deleteStaleReservation(ctx, oldID)
	r.logger.DebugContext(ctx, "external id updated", "external_id.entity_id", e.EntityID)
	return nil
}

// reserve creates the reservation id for e, self-healing a stale
// reservation and retrying once if the first attempt conflicts with one.
// Returns ports.ErrConflict if a different, live row genuinely owns that
// identity.
func (r *Repository) reserve(ctx context.Context, id string, e *domain.ExternalID) error {
	data, err := json.Marshal(reservation{EntityID: e.EntityID})
	if err != nil {
		return fmt.Errorf("adapters/store/externalid: marshal reservation for external id %s: %w", e.EntityID, err)
	}
	doc := datastore.Document{Collection: reservationCollection, ID: id, Data: data}

	if err := r.ds.Create(ctx, doc); err == nil {
		return nil
	} else if !errors.Is(err, ports.ErrConflict) {
		return err
	}

	if _, found, err := r.resolveIdentity(ctx, e.EntityType, e.Source, e.Value); err != nil {
		return err
	} else if found {
		return ports.ErrConflict
	}

	return r.ds.Create(ctx, doc)
}

// Delete implements ports.ExternalIDRepository. The deleted row's
// reservation is cleaned up best-effort after the ExternalID document
// itself is removed — see docs/adr/0026-external-id-get-or-create.md.
func (r *Repository) Delete(ctx context.Context, entityType domain.EntityType, entityID, source string) error {
	ctx, span := r.tracer.Start(ctx, "external_id_repository.delete", trace.WithAttributes(attribute.String("external_id.entity_id", entityID)))
	defer span.End()

	existing, err := r.inner.Get(ctx, string(entityType), entityID, source)
	if err != nil {
		return err
	}

	if err := r.inner.Delete(ctx, string(entityType), entityID, source); err != nil {
		return err
	}

	r.deleteStaleReservation(ctx, reservationID(existing.EntityType, existing.Source, existing.Value))
	r.logger.DebugContext(ctx, "external id deleted", "external_id.entity_id", entityID)
	return nil
}
