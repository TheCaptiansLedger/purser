# 0023. Job Queue: Ephemeral In-Process Job/Task/Step Tracking, Polled and Streamed

Status: Accepted

## Context

The upcoming disk-scan/identification/import pipeline (see
[0024](0024-pipeline-core.md)) needs to run long-running, multi-step
operations — scanning a directory, fingerprinting a batch of files,
identifying each against metadata providers — and needs to expose their
progress to a caller. Nothing in the codebase does this today:
[0011](0011-api-design.md) defines the RPC/service/testing conventions for
ordinary CRUD entities but has no async/long-running-operation pattern at
all, and no `internal/service` code currently runs work outside the
lifetime of a single RPC call.

The requirement, from the planning conversation, is more specific than "a
progress bar": for a scan of, say, 13 tracks, a user or the UI needs to see,
per track, what the pipeline is currently doing and what each step's result
was — not just "60% done." That means tracking has to happen at three
levels of granularity (the overall operation, each file/unit within it, and
each discrete action taken on that unit), not one aggregate counter.

This needs to work today without a new infrastructure dependency, but the
design must not foreclose adding one: **ephemeral, in-process storage now**
(lost on restart is accepted), **a persistent backend (Redis or similar)
later** without changing the shape callers depend on.

This is deliberately general-purpose infrastructure, not
pipeline-specific — any future long-running operation (a bulk delete job,
a future indexer/download feature) is a candidate consumer, which is why it
gets its own ADR rather than being folded into [0024](0024-pipeline-core.md).

## Decision

### Domain shape: `Job` → `Task` → `Step`

Three levels, matching the three levels of information a caller actually
needs:

- **`Job`** — one overall operation (e.g. "scan `/music/new-arrivals`").
  Has an `ID`, a `Kind` (open string, not a closed enum — pipeline code
  defines its own kinds; this package doesn't know what a "scan" is),
  `Status` (`pending`\|`running`\|`succeeded`\|`failed`\|`partial`),
  `CreatedAt`/`StartedAt`/`FinishedAt`, and a `Progress` computed from its
  `Task`s, not stored redundantly.
- **`Task`** — one unit of work within the job (one file, one track). Has
  an `ID`, a `Label` for display (e.g. `"03 - Bella Donna.flac"`), the same
  `Status` enum, and a list of `Step`s. `Task.Progress` is computed from
  its `Step`s the same way `Job.Progress` is computed from its `Task`s.
- **`Step`** — one discrete action taken on a `Task` (`"compute hashes"`,
  `"extract embedded tags"`, `"query AcoustID"`, `"score candidates"`).
  Has `Name`, `Status`, `StartedAt`/`FinishedAt`, and a `Message` plus an
  open `Detail map[string]string` for structured results (a matched MBID,
  a computed confidence score) — the exact piece the "what is it doing and
  what happened" requirement needs, not derivable from a log line alone.

### Where it lives: `pkg/jobqueue`, a generic engine with no Purser knowledge

The `Job`/`Task`/`Step` types and the engine that manages their lifecycle
and computes progress live in `pkg/jobqueue` — the same category of
reusable, domain-agnostic infrastructure as `pkg/fswatch` and `pkg/cache`,
targeting the same 80% coverage bar as `pkg/**` under
[0003](0003-go-testing-standards.md). It has zero imports of
`internal/domain` or any other Purser-specific package. Purser-specific job
*kinds* (a scan job, an identify job) are defined by the caller, not by this
package.

### Storage swappability lives inside `pkg/jobqueue`, not through `Datastore`

`pkg/jobqueue` defines its own small internal `Store` interface (create,
update, get, list) with an in-memory implementation shipped by default; a
future `pkg/jobqueue/redis` package can implement the same interface
without touching the engine or any caller. This deliberately does **not**
go through [0012](0012-datastore-persistence.md)'s generic `Datastore`
(Badger/SQL): that abstraction is built for durable, moderate-write-volume
kernel/module entities, and jobs are the opposite profile — ephemeral by
design, and potentially many step updates per second during an active scan.
Forcing job tracking through the same `Document`/`Index` machinery built
for `Person`/`Group`/`MusicRelease` would be the generic-abstraction misuse
[0012](0012-datastore-persistence.md)'s own self-audit warns against, not a
reuse win. This mirrors how `Datastore` itself is explicitly not a `ports`
interface ([0012](0012-datastore-persistence.md)) — a deliberate,
self-contained abstraction one level removed from the rest of the system,
not a gap.

### Hexagonal wiring: `internal/ports` stays the seam, per 0001

`pkg/jobqueue` is consumed through two narrow `internal/ports` interfaces,
split by direction (Interface Segregation, per
[0002](0002-solid-design-principles.md)):

- **`JobPublisher`** — used by pipeline/service code to create a job, add
  tasks, and push step-status updates as work happens. The pipeline never
  imports `pkg/jobqueue` directly; it depends on this port.
- **`JobReader`** — used by the API layer to fetch a job's current state
  (`Get`), list jobs (`List`), and subscribe to updates (`Watch`, see
  below). The API layer never needs the write side.

`internal/adapters/jobqueue` is the thin adapter implementing both,
wrapping a `pkg/jobqueue.Engine`. `internal/service` depends only on the
ports, per [0001](0001-hexagonal-architecture.md) — this is the same
port/adapter split every other capability in this codebase already follows,
applied to infrastructure instead of a kernel entity.

### API: both polling and streaming, from the start

A new `purser.job.v1.JobService`, per [0011](0011-api-design.md)'s
per-entity-service convention, treating `Job` as the resource even though
its backing store isn't `Datastore`-backed:

- `GetJob` — current full state (job + all tasks + all steps) in one call.
  The simple, cache-friendly path; sufficient for a UI that polls on an
  interval.
- `ListJobs` — recent/active jobs, paginated per
  [0011](0011-api-design.md)'s cursor convention.
- `WatchJob` — a server-streaming RPC pushing job/task/step updates as they
  happen. **This is the first streaming RPC in the codebase** — Connect
  supports server-streaming natively, so this needs no new framework, but
  it is a new pattern for `internal/api/connect` and worth calling out as
  a precedent-setter, not just another handler.

Both are real, supported entry points, not one built now and one deferred —
a caller that only wants to poll uses `GetJob`; a caller that wants live
updates uses `WatchJob`; both read from the same `JobReader` port.

### Telemetry and logging are a second audience on the same events, not a separate system

Every `Step` status transition does two things, not one: it updates the
job-queue record (what the UI/API reads), and it emits a structured
`slog` line (`component=jobqueue`, `job.id`, `task.id`, `step.name`) plus an
OTel span/metric, per [0007](0007-telemetry.md)/[0008](0008-structured-logging.md).
These serve different audiences (an operator reading logs/traces vs. a user
watching progress) off the identical underlying event — not two tracking
mechanisms to keep in sync.

### IDs: the same UUIDv7 shape as `domain.NewID()`, generated without importing it

`Job`/`Task`/`Step` IDs are UUIDv7, matching the k-sortable scheme
[0020](0020-server-generated-kernel-entity-ids.md) established for kernel
entities — a deliberate choice for consistency across the codebase, not an
extension of that ADR's scope (jobs aren't kernel entities; they're not
`Datastore`-backed, so 0020 doesn't literally govern them). Critically,
`pkg/jobqueue` generates these itself by calling
`github.com/google/uuid`'s `NewV7()` directly — the same underlying
algorithm `domain.NewID()` wraps — rather than importing
`internal/domain` to call `domain.NewID()`. Same ID shape, zero actual
dependency: this is what keeps "`pkg/jobqueue` has zero imports of
`internal/domain` or any other Purser-specific package" (above) literally
true rather than in tension with this section.

## Consequences

- Any future long-running operation (bulk delete, a future
  indexer/download feature) gets progress tracking for free by depending on
  `JobPublisher`/`JobReader` — this was designed generic on purpose.
- Job history does not survive a restart until a persistent `Store`
  implementation is built. A scan interrupted by a restart needs to be
  re-triggered; there is no resume-from-where-it-left-off in this pass.
- `WatchJob` being the first streaming RPC means `internal/api/connect`
  gains a pattern (stream lifecycle, client-disconnect handling) that
  every later streaming RPC will follow — worth reviewing carefully since
  mistakes here become precedent.
- Two API paths (`GetJob` and `WatchJob`) for the same underlying data is
  intentional redundancy, not indecision — different callers have
  different needs and both are cheap to support off one `JobReader` port.

## Self-Audit Checklist

1. Does any pipeline or service code import `pkg/jobqueue` directly instead
   of depending on `ports.JobPublisher`/`ports.JobReader`? If yes — fix it;
   that breaks the hexagonal seam this ADR exists to establish.
2. Does `pkg/jobqueue` import anything from `internal/domain`,
   `internal/ports`, or any other Purser-specific package? If yes — fix
   it; it must stay generic, reusable infrastructure.
3. Does any code route job/task/step persistence through
   `internal/adapters/datastore` (Badger/SQL)? If yes — stop; that was
   explicitly rejected above for the durability/write-volume mismatch.
4. Does a `Step`'s result end up only in a log line, with no corresponding
   update to the job-queue record the API/UI can read? If yes — fix it;
   logs and job-queue state must both come from the same event, not one
   in place of the other.
5. Does `internal/api/connect`'s `JobService` implementation call
   `pkg/jobqueue` directly instead of going through `internal/service` and
   the ports above? If yes — fix it, per [0011](0011-api-design.md)'s
   existing rule that handlers never bypass the service layer.
6. Is a new long-running operation elsewhere in the codebase building its
   own bespoke progress-tracking instead of depending on
   `JobPublisher`/`JobReader`? If yes — that's exactly the duplication this
   ADR exists to prevent; use the job queue.
