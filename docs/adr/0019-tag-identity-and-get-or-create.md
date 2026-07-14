# 0019. Tag Identity: (Scope, Key, Value) Uniqueness via a Reservation Document

Status: Accepted

## Context

`domain.Tag.ID` is caller-supplied, like every kernel entity's ID (see
[issue #447](https://github.com/TheCaptiansLedger/purser/issues/447) for the
open question of whether that should change kernel-wide — out of scope
here; resolved by [0020](0020-server-generated-kernel-entity-ids.md), which
makes `Tag.ID` server-generated same as every other single-ID entity and
closes the "ID collision vs. get-or-create hit" ambiguity this ADR flags
below as a real, reachable case). Nothing before this ADR enforced any uniqueness on a Tag's
`(Scope, Key, Value)` — only the SQL/Badger primary key on `(collection,
id)` was ever checked, per [0012](0012-datastore-persistence.md). Two
`CreateTag` calls with identical `Scope`/`Key`/`Value` but different
caller-supplied `id`s both succeed today, producing two Tag rows that mean
the same thing. `TagAssignment` references land on whichever ID the caller
happened to use, fragmenting "everything tagged genre:gonzo" across
duplicate rows — a real, reachable bug, not a theoretical one, since every
k6 fixture in this repo that creates a Tag reuses a literal `Key`/`Value`
across every VU/iteration and only varies the `id`.

`Category` is not part of this problem: it's documented on `domain.Tag` as
an optional, StashDB-style cosmetic grouping, is never filtered or grouped
on anywhere in the Go code, the proto, or the web UI, and no fixture in the
repo reuses one `(Key, Value)` under two different `Category` values to mean
two different things. It stays a passenger field, excluded from identity.

`Scope` (`user` vs. `metadata`, [enums.go](../../internal/domain/enums.go))
*is* part of identity — it exists specifically to distinguish a
user-applied organizational tag from a provider-sourced metadata tag, so
the same `Key`/`Value` legitimately means two different things under the
two scopes.

## Decision

### Identity is `(Scope, Key, Value)`; `CreateTag` becomes get-or-create

`CreateTag` no longer unconditionally inserts. If a Tag with the same
`(Scope, Key, Value)` already exists, that existing Tag is returned as-is —
the caller's `id` and `category` are discarded, no new proto field is added
to signal "was this newly created," and no side effect (like overwriting
`category`) happens on the hit path. This is silent, not an error, because
"the tag you asked for already exists" is the expected, common case for a
shared vocabulary, not a fault condition.

### Mechanism: a reservation document, using the existing `CreateBatch` primitive — no `Datastore`/backend changes

[0012](0012-datastore-persistence.md) already established the pattern this
reuses: rely on an atomic, constraint-based conflict check (a real
`PRIMARY KEY` violation), never a racy read-then-insert. A new collection,
`tag_key`, holds one document per unique `(Scope, Key, Value)` triple:

- `Collection: "tag_key"`
- `ID`: `scope + "\x00" + key + "\x00" + value` — the same composite-key
  `\x00`-delimiter convention `CompositeRepository[T]` already uses.
- `Data`: `{"tag_id": "<the winning Tag's ID>"}`

`Create` writes the reservation document and the Tag document together via
`Datastore.CreateBatch` (already in the interface for [0016](0016-bulk-operations.md)'s
bulk-create path — zero changes to `Datastore`, the Badger backend, or the
SQL backend). Two concurrent callers racing to create the same identity: the
loser's `CreateBatch` fails with `ports.ErrConflict` on the reservation
document's primary key, exactly as race-safe as the existing `(collection,
id)` conflict check. The loser then reads the (now-committed) reservation,
fetches the winning Tag, and returns it — this is the get-or-create hit
path, and it only runs on the already-slow conflict path, never on the
common case.

### `internal/adapters/store/tag` stops being a `Repository[T]` alias

Tag's shape is no longer "single ID, no filter" — the exact case
[0012](0012-datastore-persistence.md)'s self-audit item 7 names as *not*
belonging in `Repository[T]`. It becomes a small hand-written translator,
the same exception already carved out for `Image`
([0012](0012-datastore-persistence.md)'s "`Image` stays hand-written"
addendum — Tag is the second occurrence that addendum anticipated).
It embeds `*store.Repository[domain.Tag]` for `Get`/`List` (byte-for-byte
unchanged) and overrides `Create`/`Update`/`Delete`/`DeleteBatch` to
maintain the `tag_key` reservation alongside the `tag` document.
`internal/ports.TagRepository` and everything above it
(`internal/service`, `internal/api/connect`) are unchanged — this is
entirely an adapter-layer implementation detail.

### `UpdateTag` may still rename identity fields, but under the same constraint

An existing k6 fixture renames a Tag's `value` in place
(`test/k6/grpc/tag_test.js`), so identity fields are not made immutable.
`UpdateTag` enforces the same uniqueness: renaming into a `(Scope, Key,
Value)` already owned by a *different*, live Tag is rejected with
`ports.ErrConflict`. Renaming is implemented as delete-old-reservation,
create-new-reservation, update-Tag-document — three separate `Datastore`
calls, because `Datastore` has no cross-collection transaction primitive
(only single-collection `Create`/`CreateBatch`/`DeleteBatch`). Adding one
was considered and rejected as disproportionate to this problem — seeSelf,
"Rejected alternatives" below.

### Self-healing stale-reservation detection closes the correctness gap

Because rename and delete are not single atomic transactions, a crash
mid-operation can leave a `tag_key` document pointing at a Tag whose
`Scope`/`Key`/`Value` no longer match it (renamed away) or that no longer
exists (deleted). Every reservation lookup verifies the target Tag's
current fields actually match the reservation's identity before treating it
as a hit; a mismatch is treated as stale, the stale reservation is deleted,
and the operation (a get-or-create lookup, or a rename's new-reservation
create) retries once. This guarantees get-or-create can never hand back a
Tag whose fields don't match what was asked for — the one failure mode that
would be a silent correctness bug rather than a availability hiccup.

### `DeleteTag`/`BulkDeleteTags` clean up the reservation, best-effort

Delete order is: remove the Tag document first, then remove its `tag_key`
reservation. If the second step fails (crash, not an ordinary error), the
reservation is orphaned and blocks recreation of that identity until the
self-healing check above encounters and clears it (the next `CreateTag` or
rename attempt against that identity). This is a deliberate, bounded
availability cost, not a correctness one — see Consequences.

## Consequences

- **A narrow, crash-only race window is accepted, not solved.** The
  residual risk after self-healing is availability, not correctness: a
  process crash between deleting a Tag (or renaming it) and cleaning up its
  old reservation can block reuse of that identity until the next write
  touches it. This is the same class of risk
  [0012](0012-datastore-persistence.md) already explicitly accepted
  ("a real place a service can forget to clean up a reference — there is no
  database-level safety net"), extended to this new mechanism rather than
  a new kind of risk.
- **Deleting a shared Tag is unaffected by any of this.** Per
  [0015](0015-deletion-impact-and-composing-services.md), `TagAssignment`
  is a pure join row, not a structural foreign key — `DeleteTag` never
  blocks and always unlinks every referencing `TagAssignment`, exactly as
  before. Making Tag identity correctly shared (this ADR's whole point)
  makes that existing, correct, unconditional-unlink behavior *reachable*
  in cases where the old split-brain bug accidentally shielded callers from
  it — every k6 fixture that creates a Tag with a literal, VU-shared
  `Key`/`Value` needed its fixture value suffixed per-VU alongside this
  change, so each test flow keeps owning a Tag it can safely tear down.
- **`Category` staying out of identity is a one-way door until something
  actually needs it.** If a future use case needs the same `(Key, Value)`
  to mean two different things under two `Category`s, that's a new ADR, not
  a quiet addition to the tuple here — `Category` would need to move from
  "cosmetic passenger field" to "identity field" deliberately, with its own
  migration story for any pre-existing Tags whose `Category` is empty.
- `internal/adapters/store/tag` gains real hand-written logic
  (marshal/CreateBatch/conflict-resolution) instead of a ~15-line generic
  wrapper — the direct cost of Tag no longer fitting `Repository[T]`'s
  shape, same as `Image` already pays.

### Rejected alternatives

- **A cross-collection transaction primitive on `Datastore`.** Would make
  rename fully atomic and remove the self-healing requirement entirely, but
  is a real interface change touching both backends
  ([0012](0012-datastore-persistence.md)) for one caller's benefit — Image
  and every other entity have no current need for it. Rejected as
  premature; if a second caller needs real multi-collection atomicity,
  that's the trigger to build it, not before (same reasoning
  [0012](0012-datastore-persistence.md) already applies to when a third
  generic repository shape is worth extracting).
- **Deriving `Tag.ID` deterministically from `(Scope, Key, Value)` instead
  of a separate reservation document.** Would let the existing `(collection,
  id)` primary key do all the work with no new collection. Rejected because
  it would silently stop honoring a caller-supplied `id` for Tag alone,
  inconsistent with every other kernel entity's Create contract (see
  [issue #447](https://github.com/TheCaptiansLedger/purser/issues/447) —
  that inconsistency belongs in a kernel-wide decision, not a Tag-only
  workaround; see [0020](0020-server-generated-kernel-entity-ids.md) for
  that decision).
- **Making identity fields immutable on `UpdateTag`.** Would remove the
  rename-atomicity problem entirely. Rejected because an existing k6
  fixture already exercises renaming a Tag's `value` in place, and there's
  a real use case (correcting a typo'd tag value) it would foreclose.

## Self-Audit Checklist

1. Does anything above `internal/adapters/store/tag` (service, API,
   `internal/ports`) know the `tag_key` collection or reservation documents
   exist? If yes — fix it; this must stay entirely inside the adapter.
2. Does any reservation lookup return a Tag without verifying its current
   `Scope`/`Key`/`Value` actually match the reservation's identity first?
   If yes — fix it; that's the silent-wrong-tag correctness bug this ADR
   exists to prevent, not just an availability hiccup.
3. Does `Create`, rename (`Update` changing identity fields), or `Delete`
   ever leave a `tag_key` document and its `tag` document
   inconsistent in a way self-healing can't detect (e.g., a reservation
   whose `tag_id` points at a real, live Tag with matching fields, but that
   Tag itself has no matching reservation at all)? If yes — fix the
   ordering; every step should fail toward "orphaned reservation blocks
   reuse" (safe), never toward "two live Tags both readable for one
   identity" (split-brain, the original bug).
4. Does a new Tag-adjacent change reintroduce a caller-supplied ID with no
   uniqueness check for some *other* combination of fields, repeating this
   exact class of bug for a different entity? If yes — this ADR's pattern
   (reservation document + `CreateBatch`) is the template to reuse, not a
   one-off.
