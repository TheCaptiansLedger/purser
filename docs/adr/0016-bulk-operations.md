# 0016. Bulk Operations: Batch at the Storage Layer, Bulk Endpoints Only Where Needed

Status: Accepted

## Context

Every RPC across all 11 entities operates on one row at a time — a
deliberate consequence of [0011](0011-api-design.md)'s narrow,
SRP-per-entity design, and correct as far as it goes. But a media-library
UI has real multi-select interactions — "tag these 50 scenes," "delete
these 12 duplicate entries" — that this shape handles only by issuing N
sequential single-row RPCs. That has two real costs: no atomicity (a
failure partway through leaves some rows done and others not, with no
rollback), and N round trips where one would do, which matters at the
sizes a bulk-select UI action implies (tens to hundreds of rows, not
one).

`Datastore` ([0012](0012-datastore-persistence.md)) already writes
through a single Badger transaction or a single SQL transaction per
operation — both backends already support multi-row writes inside one
transaction. Batch support is a natural extension of machinery already in
place, not new infrastructure.

## Decision

- **`datastore.Datastore` gains batch primitives** (conceptually
  `CreateBatch`/`DeleteBatch`, exact shape decided at implementation time)
  implemented as a single transaction per call in both the Badger and SQL
  backends — real atomicity (all rows in the batch succeed or the whole
  batch rolls back) and a single round trip to the backend, not a loop
  calling the existing single-row methods N times.
- **Bulk API endpoints are added only where a real UI interaction needs
  one** — starting with bulk delete and bulk tag-assignment (the two
  named multi-select use cases), not as a blanket "every entity gets a
  `BatchCreate<Entity>`/`BatchDelete<Entity>` RPC" rollout. Most entities
  will never need one; the ones behind an actual multi-select UI action
  do.
- **Bulk endpoints stay inside the existing layering** — a bulk RPC still
  goes through a service, which still goes through a
  `ports.XRepository`/`Datastore` batch call; hexagonal boundaries don't
  get shortcut just because an operation is a batch. No handler talks to
  `Datastore` directly.
- **Partial-failure semantics default to all-or-nothing** (transactional),
  matching what `Datastore`'s existing single-row operations already
  guarantee via their own transactions. A bulk operation that needs
  best-effort/partial-success semantics instead is a deliberate,
  documented exception per call site, not the default assumption.

## Consequences

- `Datastore`'s interface grows beyond the five methods
  [0012](0012-datastore-persistence.md) defined — a deliberate, scoped
  addition, not scope creep, since it's the same interface, same both
  backends, same contract-test discipline (`datastoretest` gets batch
  coverage alongside the existing single-row cases).
  `internal/adapters/store/*` translators gain batch methods only for
  entities that actually get a bulk API endpoint — not all of them
  up front.
- All-or-nothing semantics mean a single bad row (e.g. one ID in a bulk
  delete that no longer exists) fails the entire batch by default. Call
  sites that want to skip-and-report instead must say so explicitly; the
  default stays strict so partial application isn't a silent surprise.
- This composes with [0015](0015-deletion-impact-and-composing-services.md):
  a bulk delete still needs deletion-impact accounting per row (or a
  batched impact query) before it runs — bulk doesn't bypass the
  Unlink/Cascade decision, it just needs to apply it at scale.

## Self-Audit Checklist

1. Does a new bulk API endpoint call the single-row repository method in
   a loop instead of a real `Datastore` batch operation? If yes — fix
   it; that reintroduces the no-atomicity, N-round-trip problem this ADR
   exists to avoid.
2. Was a bulk endpoint added for an entity with no actual multi-select UI
   need behind it? If yes — that's speculative work this ADR explicitly
   argues against; remove it until there's a real caller.
3. Does a bulk operation silently apply partial success (some rows
   committed, some not) without that being a deliberate, documented
   choice for that specific call site? If yes — fix it; all-or-nothing
   is the default.
4. Does a bulk delete skip deletion-impact accounting
   ([0015](0015-deletion-impact-and-composing-services.md)) that the
   single-row delete path already requires? If yes — fix it; bulk isn't
   an exception to that rule.
