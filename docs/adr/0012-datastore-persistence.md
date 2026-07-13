# 0012. Persistence: A Generic Datastore Behind Badger and SQL

Status: Accepted

## Context

Purser needs a real, persistent backing store for the 11 shared-kernel
repository ports (`internal/ports/*.go`) — today every one of them only has
an in-memory adapter (`internal/adapters/memory/*`), so nothing survives a
restart. Two backend families are required: BadgerDB (embedded, pure-Go KV)
and ANSI-SQL-compatible engines (PostgreSQL, MySQL, SQLite).

The obvious approach — a bespoke Badger adapter and a bespoke SQL adapter
per entity — means 22 hand-written implementations of CRUD+List for 11
ports, each one repeating the same key-prefix/table/pagination logic with
only the field names changed. It also means SQL schema work multiplies by
three dialects the moment real per-entity tables are involved, since
Postgres/MySQL/SQLite genuinely diverge on DDL (auto-increment strategy,
`TEXT`-as-primary-key limits, etc.), not just query syntax.

A pre-reset version of this codebase (see `internal/adapters/badger/*.go`
and `internal/adapters/db/sqlite.go` prior to the `chore!: reset codebase`
commit) took the per-entity approach for Badger and had only a SQLite
adapter, no Postgres/MySQL. That code is a useful reference for low-level
patterns (schema-version stamping, embed.FS migrations, sharded key
prefixes) but is not reused wholesale — it predates the shared-kernel
entity set and the multi-dialect requirement this ADR addresses.

## Decision

### A generic `Datastore` sits below the entity repos, not inside them

`internal/adapters/datastore.Datastore` is a narrow interface — Create,
Get, Update, Delete, List — over an opaque `Document` (collection name,
ID, JSON-encoded payload, and a flat string/string index map for
filtering). It knows nothing about `Person`, `Group`, `Image`, or any other
entity. Two implementations satisfy it: `internal/adapters/datastore/badger`
and `internal/adapters/datastore/sql`.

Each `ports.XRepository` implementation (`internal/adapters/store/<entity>`)
is a thin translator: marshal `domain.X` to JSON, call
`Datastore.Create`/`Get`/`Update`/`Delete`/`List`, unmarshal the result back.
Composite-key ports (`EntryPerson`, `ItemPerson`, `ExternalID`) build a
single joined `ID` the same way the in-memory adapters already do (e.g.
`internal/adapters/memory/entryperson`'s `\x00`-joined key) and pass the
same fields into `Document.Index` for filtered `List`. `Datastore` reuses
`ports.ErrNotFound`/`ports.ErrConflict` directly — no error translation
layer.

This is **not** a `ports` interface: `internal/ports/*` stays reserved for
what `internal/service` depends on, per [0001](0001-hexagonal-architecture.md).
`Datastore` is consumed only by `internal/adapters/store/*`, one level of
indirection inside the adapter layer. The service layer never knows
`Datastore` exists.

### The SQL backend stores documents, not normalized per-entity tables

Both backends implement the *same* generic `Document` model — the SQL
backend does not fall back to per-entity tables. Concretely, two
entity-agnostic tables:

```sql
CREATE TABLE IF NOT EXISTS documents (
    collection VARCHAR(64)  NOT NULL,
    id         VARCHAR(255) NOT NULL,
    data       TEXT NOT NULL,
    index_json TEXT NOT NULL DEFAULT '{}',
    PRIMARY KEY (collection, id)
);

CREATE TABLE IF NOT EXISTS document_index (
    collection  VARCHAR(64)  NOT NULL,
    index_key   VARCHAR(64)  NOT NULL,
    index_value VARCHAR(255) NOT NULL,
    id          VARCHAR(255) NOT NULL,
    PRIMARY KEY (collection, index_key, index_value, id)
);
```

`VARCHAR(n)`, not `TEXT`, on every key column specifically because MySQL
cannot index or primary-key a `TEXT` column without a prefix-length
workaround — this keeps the DDL identical, unmodified, across
Postgres/MySQL/SQLite. This is the entire schema; adding a new entity
requires zero new migrations in either backend, in perpetuity, which is the
whole point.

`documents` alone gives efficient, indexed, cursor-paginated *unfiltered*
`List` (a `PRIMARY KEY`-ordered range scan on `id`) for the 6 of 11 entities
that have no filter arguments. `document_index` exists specifically so
*filtered* `List` (Image by owner; the three composite-key entities) is
also a real indexed lookup — per filter key, then intersect the resulting
ID sets — rather than a full-collection scan filtered in application code.
Both backends maintain `document_index` transactionally alongside the
primary row on Create/Update/Delete. The Badger backend mirrors this with
prefix-scannable secondary-index keys
(`idx\x00{collection}\x00{key}\x00{value}\x00{id}`), the same pattern the
pre-reset `internal/adapters/badger/person.go` used for its own role/link
indexes.

### SQL dialect handling is placeholder-rewriting and error-code mapping, not three query trees

`database/sql` already unifies the Go calling convention (`Open`/`Exec`/
`Query`/transactions) across drivers — that part needed no per-dialect
code. What it does not unify:

- **Placeholder syntax.** Every one of the ~10 hand-written statements
  (against `documents` and `document_index`) is written once with `?`
  placeholders. A small `rebind(query string, dialect Dialect) string`
  helper rewrites `?` → `$1, $2, ...` only for Postgres; MySQL and SQLite
  accept `?` unmodified.
- **Duplicate-key error shape.** Mapping a `Create` conflict to
  `ports.ErrConflict` needs a 3-case `isConflict(dialect Dialect, err
  error) bool` (Postgres `pgconn.PgError.Code == "23505"`, MySQL
  `mysqlErr.Number == 1062`, SQLite's constraint-error string) — relying on
  the `PRIMARY KEY` constraint itself rather than a read-then-insert check,
  which would be racy under concurrent writers. `Update`/`Delete`
  not-found detection uses `sql.Result.RowsAffected() == 0`, which is
  already driver-agnostic.

Drivers: `github.com/jackc/pgx/v5/stdlib` (Postgres), `github.com/go-sql-driver/mysql`
(MySQL), `modernc.org/sqlite` (SQLite — pure Go, no CGo, same choice the
pre-reset code made, keeps distroless/cross-compile builds simple).

### Addendum: `Repository[T]` for single-ID entities

The first entity translator (`internal/adapters/store/person`) was
initially hand-written in full — constructor, telemetry setup, Option
pattern, five CRUD methods — mirroring the shape every
`internal/adapters/memory/<entity>` package already uses. But six of the
eleven ports (`PersonRepository`, `GroupRepository`, `ItemRepository`,
`TagRepository`, `LibraryEntryRepository`, `MediaFileRepository`) share an
*identical* method shape — `Create(*T)`, `Get(id) *T`, `Update(*T)`,
`Delete(id)`, `List(pageSize, pageToken) ([]*T, string, error)` — differing
only in `T`. Hand-writing that translator six times is avoidable
duplication once one shared `Datastore` exists underneath all of them.

`internal/adapters/store.Repository[T]` is the shared generic
implementation of that shape. Each entity gets a thin wrapper package
(e.g. `internal/adapters/store/person`, ~15 lines) that pins `T`, the
collection name, and an `idOf func(*T) string` (injected rather than
required via a method on `T`, so `internal/domain` stays untouched and the
generic package makes no assumption about a domain type's field names),
and returns the result typed as that entity's specific port:

```go
func New(name string, ds datastore.Datastore, opts ...Option) (ports.PersonRepository, error) {
    return store.New(name, "person", ds, func(p *domain.Person) string { return p.ID }, opts...)
}
```

Go's structural interface satisfaction means `*store.Repository[domain.Person]`
satisfies `ports.PersonRepository` without either package needing to know
the other exists — `internal/ports`, `internal/service`, and
`internal/api/connect` are completely unaffected; they still see one
narrow, named interface per entity, per
[0001](0001-hexagonal-architecture.md)/[0002](0002-solid-design-principles.md)/
[0011](0011-api-design.md). This is purely an adapter-layer implementation
detail of *how* a translator is built, not a new decision about the
service/API/port boundaries.

The composite-key ports (`EntryPersonRepository`, `ItemPersonRepository`,
`ExternalIDRepository`) and the filtered port (`ImageRepository`) do not
fit this shape (different `Get`/`Delete` signatures, `List` filter
arguments) and keep their own hand-written or differently-shaped
translators.

## Consequences

- Adding a new shared-kernel or module entity to either persistent backend
  is additive: write the `internal/adapters/store/<entity>` translator
  against the existing `Datastore` interface, zero new migrations, zero new
  Badger key-prefix design. This is the Open/Closed property
  [0001](0001-hexagonal-architecture.md)/[0002](0002-solid-design-principles.md)
  already require, extended to the persistence layer itself.
- **No DB-enforced referential integrity.** No foreign keys, no cascade
  deletes, in either backend. Cross-entity cleanup (e.g. deleting a Person
  whose EntryPerson/ItemPerson links should also go) is an explicit
  service-layer responsibility. This matches [0011](0011-api-design.md)'s
  existing stance that composition lives above individual entity ports, but
  it is a real place a service can forget to clean up a reference — there
  is no database-level safety net.
- **No SQL JOINs across entities.** `documents`/`document_index` cannot
  answer "all LibraryEntries with Person X" in one query; that requires
  multiple round trips through separate repos. This is consistent with
  [0011](0011-api-design.md)'s explicit position that composed views are a
  separate, future, explicitly-composing service — not a capability this
  ADR needed to provide — but it is a one-way door: there is no path back
  to relational query pushdown without a schema migration off this design.
- **External SQL tooling loses entity-shaped tables.** `psql`/Metabase/
  Grafana pointed directly at the Postgres/MySQL/SQLite database sees
  `documents`/`document_index`, not `persons`/`groups`/`items`. Ad hoc SQL
  reporting against named columns is not available without going through
  the application.
- `List` filtering is bounded by however many keys are indexed in
  `Document.Index` for a given entity — a filter dimension an entity repo
  never populates cannot be queried efficiently later without a backfill.
  Entity repos are responsible for indexing every field their port's `List`
  signature actually filters on; this is checked in review, not enforced
  by the type system.
- One migration set (not three dialect-specific trees) and one Badger
  key-space design serve every current and future entity — the persistence
  layer's size no longer scales with the number of entities, only with the
  number of backends (two).

## Self-Audit Checklist

Run after any change to `internal/adapters/datastore/**` or
`internal/adapters/store/**`:

1. Does any `internal/adapters/store/<entity>` package import
   `internal/adapters/datastore/badger` or `internal/adapters/datastore/sql`
   directly, instead of depending on the `datastore.Datastore` interface? If
   yes — fix it (breaks backend substitutability).
2. Does `internal/adapters/datastore` (the interface/`Document` package
   itself) import `internal/domain` or know about any specific entity
   shape? If yes — fix it; it must stay entity-agnostic.
3. Does a new or changed port's `List` filter on a field that isn't written
   into `Document.Index` by its entity repo? If yes — the filtered list will
   silently fall back to scan-and-discard-nothing (return everything) or
   miss rows — fix the indexing before merging.
4. Did adding a new entity's `store` package require *any* change to
   `internal/adapters/datastore/badger` or `internal/adapters/datastore/sql`
   (beyond the migration already existing)? If yes — the entity-agnostic
   boundary has been broken; the fix belongs in the entity's own translator,
   not the shared backend.
5. Does any SQL statement added to the `sql` backend hard-code `$1`-style or
   `?`-style placeholders instead of going through `rebind`? If yes — fix it,
   it will break on the other dialect family.
6. Does a new single-ID entity translator (`Create(*T)`/`Get(id)`/
   `Update(*T)`/`Delete(id)`/`List(pageSize, pageToken)`, no filter args)
   get hand-written instead of instantiating `internal/adapters/store.Repository[T]`?
   If yes and there's no genuine reason it can't fit that shape — collapse
   it to the thin-wrapper pattern in `internal/adapters/store/person`.
7. Does a composite-key or filtered entity get forced into
   `Repository[T]` by working around its shape (e.g. faking a single ID by
   string-joining key parts and losing the filter fields)? If yes — write
   its own translator instead; `Repository[T]` is only for the six ports
   with the exact single-ID, unfiltered shape described above.
