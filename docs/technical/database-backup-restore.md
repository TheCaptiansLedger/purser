# Database Backup/Restore — Mechanism Design

> Status: proposal, not yet implemented. Written to unblock #612
> (`DatabaseService`: `GetDatabaseInfo`/`Backup`/`Restore`) and #613 (web
> Database tab). Governed by
> [0012](../adr/0012-datastore-persistence.md) — this note extends that
> ADR's design with a new capability, it does not change
> `datastore.Datastore`'s existing shape.

## 1. The gap

Neither `internal/adapters/datastore/badger` nor `.../sql` has any
backup/restore path today. An operator running Purser has no way to
snapshot their library before an upgrade, or recover after a bad restore
target, short of stopping the process and copying whatever files their
backend happens to use — which isn't a mechanism this codebase offers or
documents.

## 2. Decision: one portable artifact, produced two ways

Both backends implement the same generic `Document{Collection, ID, Data,
Index}` model ([0012](../adr/0012-datastore-persistence.md)'s central
decision). Backup/restore rides that symmetry instead of inventing a
backend-specific artifact: **the backup file is a single, ordered,
version-stamped stream of `Document` envelopes** — Badger and SQL each
produce and consume the exact same format, so one restore code path
serves both, and a backup taken against one backend is byte-for-byte
loadable by the other.

### 2.1 Format

Newline-delimited JSON (JSONL), one object per line, so a backup can be
streamed and restored without buffering the whole database in memory —
this matters directly for #612's `Backup` (stream bytes down) and
`Restore` (upload, apply).

Line 1 is a version header, not a document:

```json
{"purser_backup_version": 1}
```

Every subsequent line is one document:

```json
{"collection": "person", "id": "01930...", "data": {...}, "index": {"owner_type": "person"}}
```

`data` carries the same already-JSON-encoded entity `datastore.Document.Data`
holds; `index` is `Document.Index` verbatim. Lines are ordered by
`(collection, id)` — both backends can produce this order cheaply (see
below), and restore relies on it for predictable, resumable batching, not
for correctness.

Restore checks the header's version before reading anything else and
rejects a stream it doesn't recognize (a mismatch means either a much
older or much newer Purser wrote it) rather than guessing at a
partially-compatible shape.

### 2.2 Badger: a raw keyspace walk, not a directory/value-log copy

Badger's key layout (`internal/adapters/datastore/badger/keys.go`) is
fully prefix-scannable: every document lives under `doc\x00{collection}\x00{id}`,
every secondary-index entry under `idx\x00...`, and the schema-version
marker is the single unrelated key `\x00schema`. That means **one**
`db.View` transaction — Badger's own MVCC point-in-time snapshot, safe to
take while the process keeps serving writes — prefix-scanning
`doc\x00` yields every document across every collection in one consistent
pass, with `collection` and `id` recovered by splitting the key on `\x00`.
No enumeration of "which collections exist" is needed; the walk discovers
them from the keys it encounters.

This is deliberately **not** a filesystem copy of Badger's data
directory or value log, which the issue that opened this task originally
proposed. Rejected for three reasons:

1. **Consistency.** A raw copy of an open BadgerDB's on-disk files while
   writes are in flight is not guaranteed to produce a valid, loadable
   database — Badger's own guidance for a live backup is to use its
   logical KV-stream API (or stop the process first), not copy files
   out from under the LSM tree and background compaction.
2. **Portability.** A directory/value-log copy is Badger's own internal
   on-disk format, coupled to the exact BadgerDB version that wrote it,
   and useless as input to the SQL backend. The keyspace walk instead
   produces the same generic `Document` stream SQL does — one restore
   path, and a backup taken on Badger can seed a SQL instance (or vice
   versa) if an operator ever migrates backends.
3. **No new registry.** A directory copy is at least backend-symmetric
   with "just copy the files," but a *selective* Badger export (e.g.
   skipping stale/GC'd value-log garbage) would need a hardcoded list of
   collection names to iterate via the existing `List` API — new
   bookkeeping that goes stale the moment an entity is added, the same
   failure mode [0012](../adr/0012-datastore-persistence.md)'s own
   self-audit already warns about for index coverage. The raw prefix
   scan needs no such list.

### 2.3 SQL: a full-table walk of `documents`, ordered

```sql
SELECT collection, id, data, index_json
FROM documents
ORDER BY collection, id
```

One query, streamed via `rows.Next()` rather than buffered — `documents`
already carries everything a `Document` needs (`index_json` mirrors
`Document.Index`). `document_index` and `schema_migrations` are
deliberately not read: `document_index` is a derived secondary index,
fully reconstructible from `index_json` on restore via the normal write
path, and `schema_migrations` is process/backend state, not library data.

## 3. Restore

Restore is destructive and always starts from the same state, whether
the target is a brand-new empty instance or an existing one being rolled
back: clear the datastore's documents, then replay the stream.

1. **Clear.** Badger: delete every `doc\x00`/`idx\x00` key. SQL:
   `DELETE FROM document_index; DELETE FROM documents;` (plain `DELETE`,
   not `TRUNCATE` — SQLite has no `TRUNCATE`, keeping one statement form
   across all three dialects per [0012](../adr/0012-datastore-persistence.md)'s
   existing `rebind` convention). `schema_migrations` is untouched —
   restore assumes it's running against a datastore that's already been
   through `Open()`'s migrations, not a bare database.
2. **Replay.** Read the stream line by line (after validating the
   version header), decode each line back into a `datastore.Document`,
   and hand bounded-size chunks (e.g. 500 documents) to the existing
   `Datastore.CreateBatch` ([0016](../adr/0016-bulk-operations.md)) —
   already atomic per chunk, already maintains `document_index`
   transactionally. **No new write-path code is needed for restore** —
   only the two backend-specific walkers above are new; loading a
   decoded stream back in reuses machinery that already exists and is
   already tested.

Restore's actual "requires restart" orchestration (staging an uploaded
backup, applying it, forcing the process to reopen the datastore
cleanly) is #612's concern, not this note's — the contract this note
fixes is just: given a validated JSONL stream, `CreateBatch` chunks are
sufficient to reload it.

## 4. The new seam: `ports.DatabaseAdmin`

`datastore.Datastore` stays exactly as [0012](../adr/0012-datastore-persistence.md)
defined it — this doesn't add `Backup`/`Restore` methods there, because
`Datastore` is consumed only by `internal/adapters/store/*` translators
and knows nothing about raw keyspace/table access. Instead, a new,
narrow `internal/ports.DatabaseAdmin` interface —

```go
type DatabaseAdmin interface {
    Info(ctx context.Context) (DatabaseInfo, error)
    Backup(ctx context.Context, w io.Writer) error
    Restore(ctx context.Context, r io.Reader) error
}
```

— implemented by two new small adapters, `internal/adapters/database/badger`
and `internal/adapters/database/sql`, each holding both the raw handle
(for the backup walk) and a reference to the corresponding
`datastore.Datastore` (for restore's `CreateBatch` calls). This mirrors
[0023](../adr/0023-job-queue.md)'s `JobPublisher`/`JobReader` precedent —
a genuinely new capability gets its own thin port rather than widening an
existing one, keeping Interface Segregation ([0002](../adr/0002-solid-design-principles.md))
intact: `DatabaseService` (#612) depends on `DatabaseAdmin` alone, never
on `Datastore` or any entity repository port.

`DatabaseInfo` itself (driver, version, storage size, per-collection
counts) is #612's shape to define — worth noting here only because the
same raw walk this note designs for `Backup` computes per-collection
counts as a free side effect, with no separate pass needed.

## 5. Explicitly out of scope for this note

- Proto/RPC message shapes and the Connect service definition for
  `Backup`/`Restore`/`GetDatabaseInfo` — #612's job.
- How `Backup`'s stream maps onto a Connect server-streaming RPC, or how
  `Restore`'s upload is chunked over the wire — implementation detail of
  #612, not a data-format decision.
- Compression of the backup artifact (e.g. gzip) — an orthogonal
  transport concern; nothing above assumes an uncompressed stream.
- The "requires restart" restore orchestration (where an uploaded backup
  is staged, how the process is told to reopen cleanly) — #612.
- Scheduled/automatic backups — not requested anywhere in #596's scope.
