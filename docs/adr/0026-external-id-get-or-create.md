# 0026. ExternalID Get-or-Create: Extending the Reservation-Document Pattern to (EntityType, Source, Value)

Status: Accepted

## Context

[0019](0019-tag-identity-and-get-or-create.md) fixed a real race: two
concurrent `CreateTag` calls with the same `(Scope, Key, Value)` but
different caller-supplied IDs both succeeded, producing two rows meaning
the same thing. Its own self-audit named this explicitly as a template,
not a one-off: "does a new Tag-adjacent change reintroduce a
caller-supplied ID with no uniqueness check for some *other* combination
of fields... this ADR's pattern is the template to reuse."

Music's identification pipeline ([0025](0025-music-identification-confidence-scoring.md),
specifically the decide/persist step) hits exactly that case. Persisting a
resolved MusicBrainz artist means "find the `LibraryEntry` already linked
to this MBID, or create one" — and per
[0021](0021-music-domain-model.md)'s explicit decision, an artist's or
release group's MusicBrainz ID isn't a plain field on the entity; it's a
row in the shared `ExternalID` join
(`EntityType=library_entry, Source=mbz, Value=<mbid>`). Two concurrent
scans (or a scan and a manual "accept candidate" action) resolving the
same not-yet-imported artist at the same time can both find no existing
`ExternalID` row and both create a new `LibraryEntry` — the same class of
duplicate-identity bug 0019 fixed for Tag, just one join-table hop removed.

[0021](0021-music-domain-model.md) separately already flagged, in
passing, that `ExternalIDRepository` "has no lookup-by-value" — finding an
entity by its external identifier isn't possible today at all, uniqueness
race aside. This ADR fixes both at once: the missing capability and the
race it would otherwise still have.

**Why this isn't a straight copy of 0019's mechanism.** Tag's fix works
because Tag *is* the entity with the unique identity — one `CreateBatch`
call writes the reservation and the Tag document together, atomically, so
a losing caller never created anything that needs cleaning up. `ExternalID`
is different: the identity being protected here (`EntityType, Source,
Value`) belongs to the `ExternalID` row, but the thing a caller actually
wants get-or-create semantics for is the *owning entity*
(`LibraryEntry`/`Group`) — a different document, in a different
repository, that `ExternalIDRepository` has no visibility into and, per
[0001](0001-hexagonal-architecture.md), shouldn't. That composition
wrinkle is the actual new content of this ADR, not a restatement of 0019.

## Decision

### `ExternalIDRepository` gains `GetByValue`

```
GetByValue(ctx, entityType domain.EntityType, source domain.ExternalIDSource, value string) (*domain.ExternalID, error)
```

Returns `ports.ErrNotFound` if no `ExternalID` row of that type/source
currently owns that value. This is the lookup 0021 noted was missing,
built as a side effect of the reservation mechanism below rather than as
a separate piece of work.

### `ExternalIDRepository.Create` becomes get-or-create on its own identity, exactly like 0019

A new reservation collection, `external_id_value`, holds one document per
unique `(EntityType, Source, Value)`:

- `ID`: `entityType + "\x00" + source + "\x00" + value` — the same
  `\x00`-delimiter convention 0019 and `CompositeRepository[T]` already use.
- `Data`: `{"entity_id": "<the winning ExternalID row's EntityID>"}`.

`Create` writes the reservation and the `ExternalID` document together via
`Datastore.CreateBatch` — no `Datastore`/backend changes, identical
mechanism to 0019. A losing caller's `CreateBatch` fails with
`ports.ErrConflict` on the reservation's primary key; it then reads the
committed reservation and **mutates its argument in place** to the
winning row's fields (`*e = *existing`), the same convention 0019 uses for
`Tag` — the caller inspects `e.EntityID` after `Create` returns to learn
who actually won.

Self-healing is identical to 0019's: before trusting a reservation as a
hit, verify the `ExternalID` row it points at still exists with matching
`EntityType`/`Source`/`Value`. A stale reservation (left by a crash
mid-delete) is cleared and the caller proceeds as if none existed.

### The composition pattern: speculative create, then reconcile

Because the owning entity (`LibraryEntry`, `Group`, ...) lives in a
different repository, protecting `ExternalID`'s own identity doesn't by
itself stop two callers from both creating a duplicate `LibraryEntry`. The
pattern every "get-or-create an entity by external ID" caller follows:

1. `GetByValue(ctx, entityType, source, value)`. Found → fetch and use
   that entity. Done — this is the common, fast path.
2. Not found → **speculatively** create the new entity via its own
   repository (`libraryEntryRepo.Create`, a fresh server-generated ID —
   always succeeds, `LibraryEntry` itself has no competing uniqueness
   constraint here).
3. Call `externalIDRepo.Create` linking that entity to the value. If it
   returns with `e.EntityID` equal to the entity just created in step 2 —
   this caller won the race, done. If `e.EntityID` differs — this caller
   *lost*: **delete the entity speculatively created in step 2** (safe —
   it has a fresh ID nobody else could have referenced yet) and use the
   winner's entity (fetched via `e.EntityID`) instead.

This is a deliberate, narrow exception to "no speculative work" — the
window where a losing caller's `LibraryEntry` briefly exists and then gets
deleted is bounded, self-contained (only the loser can see or reference
it), and cheaper than trying to make entity creation and external-ID
linking a single cross-repository transaction (which `Datastore` doesn't
support, per [0012](0012-datastore-persistence.md), and 0019 already
rejected building for one caller's benefit).

### `Update` may still change `Value`, under the same reservation constraint

`ExternalIDRepository.Update` already changes `Value` in place (the port's
own doc comment has always said "only Value is mutable via Update"), and
`UpdateExternalID` is a live, reachable RPC that calls it. Once `Value` is
part of the reservation identity, an `Update` that moves `Value` without
also moving the reservation would leave the entity's *current* value
unreservationed — `GetByValue` on the value the row actually holds today
would wrongly report `ports.ErrNotFound`. This is the same class of problem
[0019](0019-tag-identity-and-get-or-create.md) solved for Tag's rename case
("`UpdateTag` may still rename identity fields"), applied here: `Update`
enforces the same uniqueness `Create` does when `Value` changes, following
the identical reserve-new/delete-old ordering 0019's `Tag.Update` uses —
reserve the new `(EntityType, Source, NewValue)` identity first (returning
`ports.ErrConflict` if a different, live row already owns it), then update
the `ExternalID` document, then best-effort delete the old reservation. An
`Update` that leaves `Value` unchanged skips reservation work entirely.

### `Delete`/deletion-impact cleanup

An entity's composing deletion service (per
[0015](0015-deletion-impact-and-composing-services.md)) already removes
its `ExternalID` referrer rows. `ExternalIDRepository.Delete` cleans up
the `external_id_value` reservation best-effort after removing the
`ExternalID` document itself — same ordering, same accepted crash-window
risk, as 0019's `Tag` delete path.

## Consequences

- `ExternalIDRepository`'s port grows one method (`GetByValue`); its
  `Create` contract changes from "always inserts" to "get-or-create,
  mutates its argument on a hit" — a real behavior change every existing
  caller of `ExternalIDRepository.Create` needs to be compatible with
  (idempotent re-linking becomes safe where it wasn't specified before).
- `Update` also gains reservation-moving logic when `Value` changes, closing
  a gap this ADR's first draft missed: without it, `UpdateExternalID` (a
  live RPC) could silently leave a row unreachable via `GetByValue` at its
  own current value. See "`Update` may still change `Value`" above.
- Every future module doing "look up or create an entity from a
  provider's external identifier" (not just Music) reuses this — the
  composition pattern above is the template, the same way 0019 asked
  future Tag-adjacent problems to reuse its mechanism.
- A caller that forgets step 2's cleanup on loss leaks an orphaned entity
  with no `ExternalID` link — worth a self-audit item, since unlike 0019's
  stale-reservation case (self-healing, bounded), an orphaned *entity* has
  no automatic cleanup mechanism here.
- Same narrow, crash-only race window 0019 accepts (a crash between
  creating the `ExternalID` row and later deleting it, before the
  reservation cleanup runs) is extended here, not a new kind of risk.

## Self-Audit Checklist

1. Does any code call `ExternalIDRepository.Create` expecting it to always
   insert a new row, without checking whether its argument was mutated to
   a pre-existing row afterward? If yes — fix it; that's exactly the
   get-or-create contract this ADR establishes.
2. Does a caller composing an entity with an `ExternalID` link skip the
   speculative-create-then-reconcile pattern, instead creating the entity
   and the `ExternalID` row with no reconciliation step at all? If yes —
   fix it; that's this ADR's whole point, reintroducing 0019's bug one
   hop removed.
3. Does a losing caller in the composition pattern fail to delete its
   speculatively-created entity, leaving an orphaned row with no
   `ExternalID` link and no path to discover it later? If yes — fix it;
   unlike a stale reservation, an orphaned entity doesn't self-heal.
4. Does any reservation lookup return a hit without verifying the target
   `ExternalID` row's current `EntityType`/`Source`/`Value` actually match
   first? If yes — fix it, same correctness bug 0019's self-audit names.
5. Does a new module facing this same "get-or-create by external
   identifier" problem build its own bespoke mechanism instead of reusing
   `GetByValue`/`Create`'s get-or-create contract and the composition
   pattern here? If yes — that's the reuse this ADR (and 0019 before it)
   exists for; point them here first.
6. Does `Update` change `Value` without moving the `external_id_value`
   reservation to match? If yes — fix it; that leaves the row unreachable
   via `GetByValue` at its own current value, the exact gap this ADR's
   "`Update` may still change `Value`" section closes.
