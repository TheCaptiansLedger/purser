// Package tag is the datastore-backed adapter for the ports.TagRepository
// port. Unlike the other six single-ID entities, Tag must enforce
// (Scope, Key, Value) uniqueness across the whole collection, not just its
// own caller-supplied ID — a shape store.Repository[T] doesn't cover, so
// this is a hand-written translator, the same exception
// docs/adr/0012-datastore-persistence.md already carves out for Image. See
// docs/adr/0019-tag-identity-and-get-or-create.md for the full design.
package tag

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
	collection = "tag"

	// reservationCollection holds one document per unique (Scope, Key,
	// Value) identity, pointing at the Tag that currently owns it — see
	// docs/adr/0019-tag-identity-and-get-or-create.md.
	reservationCollection = "tag_key"

	// keySeparator matches the composite-key convention
	// store.CompositeRepository[T] already uses.
	keySeparator = "\x00"

	instrumentationName = "purser/internal/adapters/store/tag"
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
	TagID string `json:"tag_id"`
}

// Repository is the datastore-backed ports.TagRepository adapter. See
// docs/adr/0019-tag-identity-and-get-or-create.md.
type Repository struct {
	tags *store.Repository[domain.Tag]
	ds   datastore.Datastore

	logger *slog.Logger
	tracer trace.Tracer

	creates metric.Int64Counter
	updates metric.Int64Counter
	deletes metric.Int64Counter
}

var _ ports.TagRepository = (*Repository)(nil)

// New constructs a named ports.TagRepository backed by ds.
func New(name string, ds datastore.Datastore, opts ...Option) (*Repository, error) {
	tags, err := store.New(name, collection, ds, idOf, opts...)
	if err != nil {
		return nil, err
	}

	r := &Repository{
		tags:   tags,
		ds:     ds,
		logger: tags.Logger().With("component", "adapters.store."+reservationCollection),
		tracer: tags.Tracer(),
	}

	meter := tags.MeterProvider().Meter(instrumentationName)
	if r.creates, err = meter.Int64Counter(collection+"_repository.creates", metric.WithDescription(collection+" records created")); err != nil {
		return nil, fmt.Errorf("adapters/store/tag: creating creates counter: %w", err)
	}
	if r.updates, err = meter.Int64Counter(collection+"_repository.updates", metric.WithDescription(collection+" records updated")); err != nil {
		return nil, fmt.Errorf("adapters/store/tag: creating updates counter: %w", err)
	}
	if r.deletes, err = meter.Int64Counter(collection+"_repository.deletes", metric.WithDescription(collection+" records deleted")); err != nil {
		return nil, fmt.Errorf("adapters/store/tag: creating deletes counter: %w", err)
	}

	return r, nil
}

func idOf(t *domain.Tag) string { return t.ID }

func reservationID(scope domain.TagScope, key, value string) string {
	return string(scope) + keySeparator + key + keySeparator + value
}

// Get implements ports.TagRepository. Identity uniqueness doesn't affect
// reads, so this delegates straight to the embedded single-ID translator.
func (r *Repository) Get(ctx context.Context, id string) (*domain.Tag, error) {
	return r.tags.Get(ctx, id)
}

// List implements ports.TagRepository — see Get.
func (r *Repository) List(ctx context.Context, pageSize int, pageToken string) ([]*domain.Tag, string, error) {
	return r.tags.List(ctx, pageSize, pageToken)
}

// Create implements ports.TagRepository as get-or-create on
// (t.Scope, t.Key, t.Value) — see
// docs/adr/0019-tag-identity-and-get-or-create.md. If a Tag with that
// identity already exists, t is mutated in place to the pre-existing Tag
// (its incoming ID/Category are discarded) and Create returns nil; a
// genuinely new identity is created as given.
func (r *Repository) Create(ctx context.Context, t *domain.Tag) error {
	ctx, span := r.tracer.Start(ctx, "tag_repository.create", trace.WithAttributes(
		attribute.String("tag.id", t.ID), attribute.String("tag.key", t.Key), attribute.String("tag.value", t.Value),
	))
	defer span.End()

	if err := r.createOnce(ctx, t); err == nil {
		r.creates.Add(ctx, 1)
		r.logger.DebugContext(ctx, "tag created", "tag.id", t.ID)
		return nil
	} else if !errors.Is(err, ports.ErrConflict) {
		return err
	}

	existing, found, err := r.resolveIdentity(ctx, t.Scope, t.Key, t.Value)
	if err != nil {
		return err
	}
	if found {
		span.SetAttributes(attribute.Bool("tag.get_or_create_hit", true))
		*t = *existing
		return nil
	}

	// The reservation wasn't the cause of the conflict (or was stale and
	// has now been cleared) — retry once; a second conflict means the
	// caller's own ID collides with an unrelated Tag.
	if err := r.createOnce(ctx, t); err != nil {
		return err
	}
	r.creates.Add(ctx, 1)
	r.logger.DebugContext(ctx, "tag created", "tag.id", t.ID)
	return nil
}

// createOnce atomically writes t's reservation and Tag documents via
// CreateBatch — the race-safe, constraint-based conflict check
// docs/adr/0012-datastore-persistence.md establishes, not a
// read-then-insert.
func (r *Repository) createOnce(ctx context.Context, t *domain.Tag) error {
	tagData, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("adapters/store/tag: marshal tag %s: %w", t.ID, err)
	}
	resData, err := json.Marshal(reservation{TagID: t.ID})
	if err != nil {
		return fmt.Errorf("adapters/store/tag: marshal reservation for tag %s: %w", t.ID, err)
	}

	return r.ds.CreateBatch(ctx, []datastore.Document{
		{Collection: reservationCollection, ID: reservationID(t.Scope, t.Key, t.Value), Data: resData},
		{Collection: collection, ID: t.ID, Data: tagData},
	})
}

// resolveIdentity looks up the reservation for (scope, key, value). If it
// exists and the Tag it points at still has matching Scope/Key/Value, that
// Tag is returned. If the reservation is missing, or points at a Tag that
// no longer exists or whose fields no longer match (a stale reservation
// left by a crash mid-rename or mid-delete — see
// docs/adr/0019-tag-identity-and-get-or-create.md), any stale reservation
// is deleted and (nil, false, nil) is returned so the caller can proceed
// as if no reservation existed.
func (r *Repository) resolveIdentity(ctx context.Context, scope domain.TagScope, key, value string) (*domain.Tag, bool, error) {
	id := reservationID(scope, key, value)
	doc, err := r.ds.Get(ctx, reservationCollection, id)
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}

	var res reservation
	if err := json.Unmarshal(doc.Data, &res); err != nil {
		return nil, false, fmt.Errorf("adapters/store/tag: unmarshal reservation %s: %w", id, err)
	}

	existing, err := r.tags.Get(ctx, res.TagID)
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			r.deleteStaleReservation(ctx, id)
			return nil, false, nil
		}
		return nil, false, err
	}

	if existing.Scope != scope || existing.Key != key || existing.Value != value {
		r.deleteStaleReservation(ctx, id)
		return nil, false, nil
	}

	return existing, true, nil
}

func (r *Repository) deleteStaleReservation(ctx context.Context, id string) {
	if err := r.ds.Delete(ctx, reservationCollection, id); err != nil && !errors.Is(err, ports.ErrNotFound) {
		r.logger.WarnContext(ctx, "tag: failed to clean up a stale/orphaned reservation", "reservation.id", id, "error", err)
	}
}

// Update implements ports.TagRepository. If t's (Scope, Key, Value)
// differs from the existing Tag's, the identity is renamed under the same
// uniqueness constraint Create enforces, returning ports.ErrConflict if
// the new identity is already owned by a different, live Tag — see
// docs/adr/0019-tag-identity-and-get-or-create.md.
func (r *Repository) Update(ctx context.Context, t *domain.Tag) error {
	ctx, span := r.tracer.Start(ctx, "tag_repository.update", trace.WithAttributes(attribute.String("tag.id", t.ID)))
	defer span.End()

	existing, err := r.tags.Get(ctx, t.ID)
	if err != nil {
		return err
	}

	oldID := reservationID(existing.Scope, existing.Key, existing.Value)
	newID := reservationID(t.Scope, t.Key, t.Value)

	if oldID == newID {
		if err := r.tags.Update(ctx, t); err != nil {
			return err
		}
		r.updates.Add(ctx, 1)
		return nil
	}

	if err := r.reserve(ctx, newID, t.ID, t.Scope, t.Key, t.Value); err != nil {
		return err
	}

	if err := r.tags.Update(ctx, t); err != nil {
		return err
	}

	// Best-effort: a crash here leaves oldID orphaned, blocking reuse of
	// the freed identity until the next Create/rename against it
	// self-heals via resolveIdentity. See docs/adr/0019.
	r.deleteStaleReservation(ctx, oldID)
	r.updates.Add(ctx, 1)
	r.logger.DebugContext(ctx, "tag updated", "tag.id", t.ID)
	return nil
}

// reserve creates the reservation id for tagID/(scope, key, value),
// self-healing a stale reservation and retrying once if the first attempt
// conflicts with one. Returns ports.ErrConflict if a different, live Tag
// genuinely owns that identity.
func (r *Repository) reserve(ctx context.Context, id, tagID string, scope domain.TagScope, key, value string) error {
	data, err := json.Marshal(reservation{TagID: tagID})
	if err != nil {
		return fmt.Errorf("adapters/store/tag: marshal reservation for tag %s: %w", tagID, err)
	}
	doc := datastore.Document{Collection: reservationCollection, ID: id, Data: data}

	if err := r.ds.Create(ctx, doc); err == nil {
		return nil
	} else if !errors.Is(err, ports.ErrConflict) {
		return err
	}

	if _, found, err := r.resolveIdentity(ctx, scope, key, value); err != nil {
		return err
	} else if found {
		return ports.ErrConflict
	}

	return r.ds.Create(ctx, doc)
}

// Delete implements ports.TagRepository. The deleted Tag's reservation is
// cleaned up best-effort after the Tag document itself is removed — see
// docs/adr/0019-tag-identity-and-get-or-create.md.
func (r *Repository) Delete(ctx context.Context, id string) error {
	ctx, span := r.tracer.Start(ctx, "tag_repository.delete", trace.WithAttributes(attribute.String("tag.id", id)))
	defer span.End()

	existing, err := r.tags.Get(ctx, id)
	if err != nil {
		return err
	}

	if err := r.tags.Delete(ctx, id); err != nil {
		return err
	}

	r.deleteStaleReservation(ctx, reservationID(existing.Scope, existing.Key, existing.Value))
	r.deletes.Add(ctx, 1)
	r.logger.DebugContext(ctx, "tag deleted", "tag.id", id)
	return nil
}

// DeleteBatch implements ports.TagRepository, removing every Tag document
// in ids atomically, then best-effort cleaning up their reservations — see
// docs/adr/0019-tag-identity-and-get-or-create.md.
func (r *Repository) DeleteBatch(ctx context.Context, ids []string) error {
	ctx, span := r.tracer.Start(ctx, "tag_repository.delete_batch", trace.WithAttributes(attribute.Int("tag.count", len(ids))))
	defer span.End()

	reservations := make([]string, 0, len(ids))
	for _, id := range ids {
		existing, err := r.tags.Get(ctx, id)
		if err != nil {
			return err
		}
		reservations = append(reservations, reservationID(existing.Scope, existing.Key, existing.Value))
	}

	if err := r.tags.DeleteBatch(ctx, ids); err != nil {
		return err
	}

	for _, resID := range reservations {
		r.deleteStaleReservation(ctx, resID)
	}
	r.deletes.Add(ctx, int64(len(ids)))
	r.logger.DebugContext(ctx, "tag batch deleted", "count", len(ids))
	return nil
}
