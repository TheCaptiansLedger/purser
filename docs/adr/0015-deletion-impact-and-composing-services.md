# 0015. Deletion Impact and Composing Services

Status: Accepted

## Context

[0012](0012-datastore-persistence.md) traded away DB-enforced referential
integrity in exchange for a document store that scales additively to any
number of entities with zero per-entity migrations — an accepted,
documented cost, not an oversight. That ADR's own Consequences section
named the resulting risk directly: *"a caller that... fails... leaves an
orphaned file with nothing pointing at it... not solved here."* The same
risk applies to every entity relationship in the kernel: deleting a
`Person` today leaves `EntryPerson`, `ItemPerson`, `Image`
(`owner_type=person`), `ExternalID` (`entity_type=person`), and
`afterdark.PerformerProfile` rows all dangling, silently, with nothing in
the codebase noticing.

This is not a new problem to design from scratch. A pre-reset version of
this codebase already solved it:

```go
func (r *personRepo) DeletionImpact(_ context.Context, id string) (*domain.DeletionImpact, error) {
    impact := &domain.DeletionImpact{Mode: domain.DeletionModeUnlink, Impacts: []domain.DeletionImpactRow{}}
    // ... counts EntryPerson/ItemPerson rows referencing id, grouped by
    // content type, each row carrying a human-readable Label ...
}
```

`internal/adapters/badger/person.go` (deleted at the `chore!: reset
codebase` commit) — `DeletionImpact` returned a count of every
referencing row, grouped by type, with a display label, plus a `Mode`.
That's the exact shape a UI needs for "deleting this will unlink them
from 12 Movies and 3 TV Shows — continue?" This ADR revives that pattern
as a standing convention rather than re-deriving one.

Separately, [0011](0011-api-design.md) already named, but never built, a
category of service allowed to depend on more than one entity's port: *"A
handler assembling a composed view... is a future, separate, explicitly
composing service — never folded into a single-entity CRUD service."*
Deletion-impact computation is the first real instance of that category:
checking "what references this Person" necessarily means looking across
`EntryPerson`, `ItemPerson`, `Image`, `ExternalID`, and
`PerformerProfile` — no single entity's port can answer that about
itself without violating [0001](0001-hexagonal-architecture.md)'s
one-port-one-entity boundary.

## Decision

### The `DeletionImpact` shape, revived

```
DeletionImpact{
    Mode: Unlink | Cascade
    Impacts: [ { Kind, Count, Label }, ... ]
}
```

`Kind`/`Label` mirror the pre-reset shape: a machine-readable category
(`"item_movie"`) and a human-readable one (`"Movies"`) per referencing
type found. Computing this is "call `List` with an index filter for this
ID, once per known referrer type, and count" — the exact same
`Document.Index`-backed filtered `List` this codebase already has;
nothing new needed at the storage layer.

### Composing services own this, not the entity's own repository/service

A new composing service per deletable entity (e.g. a `PersonDeletion`
concern) is the thing allowed to depend on `EntryPersonRepository`,
`ItemPersonRepository`, `ImageRepository`, `ExternalIDRepository`, and
`PersonRepository` together. `internal/service/person.go` itself stays
single-port, per [0011](0011-api-design.md)'s existing "no God service"
rule — this is genuinely the exception case that ADR already carved out,
not a reason to weaken it generally.

### Two modes, `Unlink` default

- **Unlink** (default): delete only the rows that reference the target
  (the join rows), leave whatever they pointed at intact. Deleting a
  Person unlinks them from every credit; the Movies/TV Shows themselves
  are untouched.
- **Cascade** (explicit opt-in only, never the default): also delete the
  downstream records themselves. A UI must make the user actively choose
  this — "accidentally deleted 12 Movies because I deleted a Person" is
  the failure mode `Unlink`-as-default exists to prevent.

### API flow

`GetDeletionImpact(id)` is called and its result shown to the user
*before* `Delete` is invoked. `Delete` (or a mode-carrying variant of it)
then performs the unlink (always) and, only if `Cascade` was explicitly
requested, the downstream deletes — in that order, so a failure partway
through cascade-deleting leaves orphaned-but-unlinked records rather than
half-deleted-and-still-referenced ones.

## Consequences

- Every entity that can be a deletion target needs an explicit,
  hand-maintained list of "what could reference me." This does not update
  itself — adding a new entity type that references `Person` (a future
  module's own profile type, say) means remembering to add it to
  `Person`'s composing deletion service. This is a real, ongoing
  discipline cost, not automatic the way a SQL `ON DELETE CASCADE` would
  be; it is the direct, accepted trade-off for the additive-schema
  benefit [0012](0012-datastore-persistence.md) already chose.
- `Unlink`-by-default means data loss from a delete is opt-in, never
  accidental — a deliberate UX/safety choice, not just an implementation
  detail.
- This is the first standing "composing service" in the codebase;
  whatever shape it takes here (dependency injection of multiple
  repositories, no proto/API knowledge, matching every other service's
  existing convention otherwise) becomes the template the next composing
  service (a `PersonView`, an AfterDark `PerformerView`) follows, rather
  than each one inventing its own shape.

## Self-Audit Checklist

1. Does a `Delete` RPC/service method skip calling (or checking) deletion
   impact first, deleting unconditionally? If yes — fix it; silent
   unconditional delete is exactly what this ADR exists to prevent.
2. Does a composing deletion service's referrer list actually enumerate
   every port that can reference the target entity, or does it silently
   miss one (checked against that entity's own doc comments/fields for
   `OwnerType`/`EntityType`/foreign-key-shaped fields pointing at it)? If
   incomplete — fix it before merging, not after someone hits the gap.
3. Does `Cascade` ever run without the caller having explicitly requested
   it? If yes — fix it; `Unlink` must be the unconditional default.
4. Does any single-entity `internal/service/<entity>.go` file gain a
   second port dependency to support this, instead of the composing
   service pattern staying separate? If yes — that's exactly the "God
   service" [0011](0011-api-design.md) already forbids; split it out.
