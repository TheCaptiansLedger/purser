# 0020. Server-Generated Kernel Entity IDs (UUIDv7)

Status: Accepted

## Context

Every kernel entity's `Create` RPC has always accepted a caller-supplied
`ID string \`validate:"required"\`` on the wire, with no server-side ID
generation anywhere in the codebase. [Issue #447](https://github.com/TheCaptiansLedger/purser/issues/447)
surfaced this while designing the fix for Tag split-brain duplicates (see
[0019](0019-tag-identity-and-get-or-create.md)) and asked, kernel-wide,
whether that convention should change. It should: there is no legitimate
use case for a caller choosing a kernel entity's primary identity, the same
way a Kubernetes resource's UID is never client-chosen. `ExternalID`
already exists as the correct mechanism for a caller-relevant stable
identifier (StashDB UUID, TMDB ID, etc.) — caller-supplied primary IDs were
never standing in for that need.

Of the 11 kernel entities #447 enumerates, only 7 have their own single
surrogate `ID` field to generate: `Person`, `Group`, `Item`, `LibraryEntry`,
`MediaFile`, `Tag`, `Image`. The other 4 — `ExternalID`, `EntryPerson`,
`ItemPerson`, `TagAssignment` — are composite-key join rows whose identity
is entirely composed of references to other, already-server-generated IDs
(e.g. `EntryPerson`'s identity is `(LibraryEntryID, PersonID, Role)`); they
have no `ID` field of their own, so there is nothing to generate.
`Collection`/`CollectionMembership` exist only in `internal/domain` with no
port/service/API built yet, so there is no live `Create` flow to change —
this ADR's pattern applies the moment one is built, per Consequences below.

This also closes the loose end [0019](0019-tag-identity-and-get-or-create.md)
deliberately left open: with caller-supplied IDs, `TagRepository.Create`
had to distinguish "this create resolved to a pre-existing Tag" from "this
ID happens to collide with an unrelated Tag" — a real ambiguity 0019 called
out as "structurally impossible with server-generated IDs." This ADR is
that follow-through.

## Decision

### The server assigns every kernel entity's `ID` on Create; a caller-supplied value is silently discarded

For all 7 single-ID entities, `Create` no longer honors any `id` the caller
sends. This mirrors two precedents already in the codebase rather than
inventing new behavior: `PersonHandler.CreatePerson` already
unconditionally overwrites caller-supplied `AddedAt`/`UpdatedAt`, and
[0019](0019-tag-identity-and-get-or-create.md)'s get-or-create hit path
already silently discards a caller's `id` in favor of the pre-existing
Tag's. No validation error is raised if a caller sends `id` on a `Create`
request — it is simply not consulted, the same way `AddedAt` isn't.

### Algorithm: UUIDv7, via the already-vendored `github.com/google/uuid`

[RFC 9562](https://www.rfc-editor.org/rfc/rfc9562) UUIDv7 embeds a
millisecond timestamp in its high bits, making generated IDs k-sortable —
insertion order and ID order agree, which keeps Badger/SQL primary-key
locality good (unlike UUIDv4's pure randomness, which scatters inserts
across the keyspace) and makes ID order a usable proxy for creation order.
`github.com/google/uuid` is already a transitive dependency (v1.6.0, which
added `uuid.NewV7()`) via other tooling in this module — promoted to a
direct `go.mod` requirement by this change, no new dependency added. This
was chosen over `segmentio/ksuid`, which offers the same k-sortable
property but would be a new dependency for no benefit UUIDv7 doesn't
already provide, and over UUIDv4, which is simpler but gives up sort
locality for no offsetting benefit.

### Generation lives once, in `internal/domain`; enforcement lives in each service's `Create`

`internal/domain/id.go` exports `NewID() string`, a pure wrapper around
`uuid.NewV7()`. Domain is the correct layer per
[0001](0001-hexagonal-architecture.md): "every kernel entity has a
server-issued, sortable, globally unique ID" is a shared, content-type-
agnostic business rule, not a wire-transport concern (unlike proto
field-mask merging, which [0011](0011-api-design.md) deliberately keeps in
the Connect handler because it only exists there). `google/uuid` is a pure
algorithm library — no I/O, no adapter, no external SDK — so importing it
in `internal/domain` does not violate 0001's domain-purity rule.

Each of the 7 services' `Create` method calls `domain.NewID()` as its first
statement, before `Validate`:

```go
func (s *PersonService) Create(ctx context.Context, p *domain.Person) (*domain.Person, error) {
	p.ID = domain.NewID()
	if err := p.Validate(); err != nil {
		return nil, err
	}
	...
```

Putting this in the service — not only in the `internal/api/connect`
handler — means there is exactly one enforcement point no caller can
bypass, regardless of transport (gRPC, HTTP/JSON) or any future direct Go
caller of a service (e.g. a bulk-import composing service). A
handler-only implementation would have left every non-Connect caller of
`PersonService.Create` free to supply its own ID again, reintroducing
exactly the bug this ADR closes.

### `Tag` is unaffected beyond gaining the same server-generated ID

`TagService.Create` sets `t.ID = domain.NewID()` the same as every other
entity, before calling `repo.Create`. On the get-or-create hit path,
`tag.Repository.Create` already does `*t = *existing`
([0019](0019-tag-identity-and-get-or-create.md)), discarding the
newly-generated ID in favor of the pre-existing Tag's — this requires no
change to `internal/adapters/store/tag`. The retry-after-conflict fallback
in `tag.Repository.Create` (for when the reservation-document conflict
turns out not to be the cause) becomes effectively unreachable in practice
— a real ID collision on a random UUIDv7 is astronomically unlikely — but
is left in place as defense in depth rather than deleted; it costs nothing
to keep and still protects against a `google/uuid` implementation defect.

### `Update` is unaffected; `Get`/`Update`/`Delete` still take a caller-supplied `id`

Once assigned, an entity's `ID` is immutable and caller-referenced — a
caller must say *which* record to update, get, or delete. No proto/wire
schema changes anywhere: every `id` field stays on the wire messages
exactly as it is today (needed for `Get`/`Update`/`Delete`, and the
`Create` response still returns the real, server-assigned `id`). Only doc
comments change — e.g. `person.proto`'s `CreatePersonRequest` comment,
which said "Caller supplies a fully-formed Person," is corrected to note
`id` is server-assigned and ignored if sent. No `buf`/protoc regeneration
is required.

## Consequences

- **k6 fixtures lose caller-chosen, human-readable IDs.** Every affected
  entity's k6 suite generated predictable IDs
  (`k6-flow-performer-person-${suffix}`) for debuggability and reused them
  across `Create`/`Get`/`Update`/`Delete` and cross-entity references (a
  flow script threading a `Person` id into `CreatePerformerProfile` before
  the Person exists server-side). Fixed in this same change: every fixture
  now reads the real id back off the `Create` response and chains it
  through subsequent calls, rather than assuming the server honors a
  pre-computed value.
- **The Tag ID-collision ambiguity ADR 0019 flagged is now structurally
  gone**, not just unlikely — closing the loose end 0019 explicitly left
  for this ADR.
- **`Collection`, when it gets a service/API layer, must follow this same
  pattern** — `domain.NewID()` in its `Create`, not a caller-supplied `ID`
  reintroducing the problem this ADR closes for a 12th entity.
- Any future kernel entity with its own single-ID primary key is expected
  to follow this pattern by default; a caller-supplied ID for a *new*
  entity would need its own deliberate justification, not silent
  precedent-following of the pre-0020 convention.

## Self-Audit Checklist

1. Does any of the 7 services' `Create` method persist an entity without
   first calling `domain.NewID()` to assign its `ID`? If yes — fix it; a
   caller-supplied ID must never reach the repository layer.
2. Does any new kernel entity with its own single-ID primary key accept a
   caller-supplied `ID` on `Create` without a new ADR justifying the
   exception? If yes — that's a regression to the pre-0020 convention.
3. Does anything above `internal/domain` (service, API) implement its own
   ID-generation logic instead of calling `domain.NewID()`? If yes — fix
   it; generation must stay in one place.
4. Does a composite-key/join entity (`ExternalID`, `EntryPerson`,
   `ItemPerson`, `TagAssignment`, or a future one) gain its own surrogate
   `ID` field for convenience? If yes — reconsider; per this ADR's Context,
   these entities are deliberately identified by their composed references,
   not a surrogate key.
