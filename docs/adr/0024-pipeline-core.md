# 0024. Pipeline Core: Scan, Fingerprint, Identify, and Organize

Status: Accepted

## Context

[0021](0021-music-domain-model.md) built Music's CRUD layer on the shared
kernel and explicitly deferred everything about turning files on disk into
library entities: *"the import queue, per-file/per-folder tag consensus,
ranked release candidates, and the weighted multi-signal confidence
scoring... is pipeline/acquisition-core territory, not module data model,
and gets its own design pass later."* `docs/technical/shared-domain-model.md`
Part 5 flagged the same gap independently (item 2: *"Fingerprint/content-
identification as a shared pipeline concept... needs its own design pass
once the pipeline core is being defined"*). This ADR is that pass, for the
parts that are genuinely shared across every content type.

**What this ADR does not decide:** Music's actual confidence-scoring
formula and signal weights. Per the planning conversation, the prior
system built this twice (`docs/technical/v1/music-data-model.md`'s
`MusicConfidenceSignals`, and a later in-repo rework — both gone in the
`chore!: reset codebase` commit) and both failed the same way: FLAC files
with correct, complete tags still didn't land on the correct release with
confident, *differentiated* scores — candidates suspiciously converged on
similar numbers regardless of actual match quality, which points at a bug
in how signals were combined or compared, not just "the weights were
wrong." That failure mode gets a dedicated design pass once this shared
layer is settled, not a formula guessed here.

This ADR depends on [0023](0023-job-queue.md): every operation described
below is long-running and reports progress through a `Job`.

The v1 snapshot (`docs/technical/v1/`) and issues #156/#356/#357 (closed,
pre-reset) are read as **descriptive prior art** — real investigation worth
not re-deriving from scratch — never as a spec this ADR must match.

## Decision

### Data flow

```
discover ──▶ hash ──▶ fingerprint ──▶ group ──▶ identify ──▶ decide ──▶ persist ──▶ organize (optional)
```

Discovery, hashing, and grouping are content-type-agnostic. Fingerprinting
and identification are content-type-scoped capabilities fanned out to by
the shared service. Deciding, persisting, and organizing are shared
service logic acting on whatever the content-type-scoped steps produced.

### Hashing: OSHash + SHA1 always; MD5 and SHA512 are opt-in

Every discovered file always gets `OSHash` (cheap, partial-file — useful
for large video without a full read) and `SHA1`. `MD5` and `SHA512` are
computed only when explicitly enabled in config — full-file hashing on
every file is a real I/O cost, and not every deployment needs the extra
two algorithms. `domain.MediaFile` gains a `SHA512` field (`MD5`/`SHA1`/
`OSHash` already exist) — additive, same category of change as when
`SHA1` itself was added over v1's original shape. A new config component
(`internal/config`, per [0010](0010-configuration.md)'s one-struct-per-
component/`DefaultConfig()` convention) carries `EnableMD5`/`EnableSHA512`
booleans, both defaulting `false`.

### Pre-identification files are their own type, not a half-populated `MediaFile`

`domain.MediaFile.ItemID` is required — a `MediaFile` row asserts "this
file is linked to this library item," a fact that isn't true yet for a
freshly discovered file. Rather than making `ItemID` nullable (a second
kernel-struct change stacked on the `SHA512` addition, for a state that
doesn't belong on the confirmed-link type), pre-identification state lives
in a new pipeline-owned type — `domain.UnmatchedFile` — carrying path,
size, hashes, discovered-at, the computed `Fingerprint`, ranked match
candidates, and a review status (`pending`\|`matched`\|`dismissed`). A
`MediaFile` row is created only once identification actually resolves to
an `Item`. This keeps `MediaFile`'s existing meaning intact and needs zero
changes to its required fields.

### The "already known" short-circuit: hash lookup before any expensive work

Before fingerprinting or identifying a discovered file, the pipeline looks
it up by hash first — against both existing `MediaFile` rows and pending
`UnmatchedFile` rows. A hit means this is a file that moved (organized by
Purser, or moved externally) or was already queued; only its `Path` is
updated, and no fingerprint/identify/decide work runs. A miss means it's
genuinely new and proceeds through the full flow. This requires a new
hash-based lookup on the file-identity stores (`MediaFileRepository` has
none today — `Create`/`Get`/`Update`/`Delete`/`List` only). This same
mechanism is what makes the Organizer's "moved files never reimport"
requirement (below) work without inventing a second identity system —
Purser tracks files by hash, not by path, the same principle Stash uses.

### Discovery: both a watcher and an on-demand recursive scan, one code path

`pkg/fswatch` (already built) drives live discovery — each debounced
folder-unit event triggers a scan of that unit. A new RPC triggers an
on-demand recursive scan of a given root. Both converge on the identical
underlying scan operation and both create a `Job`
([0023](0023-job-queue.md)) to track it — the watcher is just an automatic
trigger for the same capability the RPC exposes explicitly, not a second
implementation.

### Grouping is a pluggable, fanned-out capability — not a music special case

How files combine into one identification unit differs by content type
(an album's tracks share a folder; a movie file usually stands alone).
Rather than hardcoding that difference, grouping is a capability
implemented per content type and fanned out to by the shared service —
the same registry pattern already validated by the `MetadataSource`
aggregator (declare what you handle via `ContentTypes()`, the service
dispatches, zero switch statements in shared code, per
[0002](0002-solid-design-principles.md)'s OCP/ISP). The default
implementation is identity grouping (one file, one unit); Music supplies a
folder-based override. The exact grouping rule Music uses (consensus tags,
disc/multi-folder handling) is deferred to the Music-specific pass this
ADR explicitly does not cover.

### Sidecar/companion assets are classified before identification, not run through it

Cover art, subtitles, `.nfo` files, and similar companions need to be
recognized during discovery and routed to attachment (e.g. cover art
becomes an `Image` row owned by the release) rather than treated as their
own unidentified media file needing a confidence score. This needs a
small, content-type-aware classification step — what counts as a sidecar
differs (subtitles matter for video, not music). The mechanism is decided
here (classify before fingerprint/identify runs); the exact per-type
classification rules are implementation detail for whoever builds each
content type's adapter.

### `FileFingerprinter`: content-type fan-out, open metadata bag

Given a discovered file (+ its hashes), extracts identification-relevant
data — embedded tags, ISRC/ISBN, an acoustic or perceptual fingerprint —
into a `Fingerprint` whose content-specific fields live in an open
`map[string]string`/`map[string]any` bag, matching the kernel's existing
"no per-module struct fields" discipline (`Item.Metadata`,
`MediaFile.Metadata`). Fanned out by `ContentTypes()`, same pattern as
grouping and `MetadataSource`.

### `ConfidenceScore`: module-owned computation, shared decision

Each content type's identifier exposes a `ConfidenceScore`-shaped
capability: given a `Fingerprint` and a candidate, return a normalized
score plus a signals breakdown. What signals exist and how they combine is
entirely module-owned — AcoustID and embedded MusicBrainz IDs for Music,
perceptual-hash distance for video, ISBN for books — because they're
genuinely different per content type. What's shared is everything
downstream of the number: the confidence *threshold* and the *decision*
(auto-import above it, land in the `UnmatchedFile` review queue below it)
live in one shared service, not duplicated per module and not decided by
the identifier itself. This mirrors v1's one correct architectural call
(score, don't decide) — kept — while the actual scoring logic is exactly
what's being redesigned, not carried forward.

### Organizer: rename + move, user-configurable, always available manually

Given an `Item`'s resolved metadata, render a Go `text/template` naming
template (per-field defaults for missing metadata) and move the file to
the resulting path, creating destination directories as needed.
**Configurable, not automatic by default**: whether organize runs
automatically after a successful match is a config toggle (new field on
the pipeline config component, alongside the hashing toggles); regardless
of that setting, organizing is always available as an explicit,
user-triggered RPC on a resolved `Item`/`MediaFile` — auto-organize being
off never removes the manual path.

**Never triggers a reimport**, by construction, not by a special case: the
organizer updates the `MediaFile`/`UnmatchedFile` row's `Path` itself as
part of the move, and even if `pkg/fswatch` independently fires on the new
location anyway, the hash-based short-circuit above recognizes it as
already-known. The same mechanism also covers files a user moves outside
Purser entirely — there's no "organizer did this" flag to check, only the
hash.

### Storage: `UnmatchedFile` gets its own hand-written translator, like `MusicRelease`

`UnmatchedFile` needs filtered listing by status and the hash lookups
above — a shape the generic single-ID repository doesn't cover, the same
situation [0021](0021-music-domain-model.md) resolved for `MusicRelease`.
It gets its own `internal/adapters/store/unmatchedfile` translator against
`Datastore`, per [0012](0012-datastore-persistence.md)'s established
"hand-written translator, not a forced-fit generic" category (`Image`,
`Tag`, `MusicRelease`). Its `ID` is server-generated via `domain.NewID()`,
per [0020](0020-server-generated-kernel-entity-ids.md) — another single-ID
entity following that pattern.

This is a deliberate contrast with [0023](0023-job-queue.md)'s `Job`,
which explicitly does **not** use `Datastore`: losing in-flight job
progress on restart is accepted, but losing the review queue (files
genuinely waiting on a human decision) would be a real regression, so
`UnmatchedFile` is durable while `Job` is ephemeral. Same ADR family,
different durability requirement, different storage decision — not an
inconsistency.

### API surface

Per-capability services, per [0011](0011-api-design.md)'s SRP-per-entity
rule applied to this domain's actual seams (no single "PipelineService"
doing everything):

- A scan-trigger RPC (on the same service `pkg/fswatch`-driven scans also
  invoke internally) — returns a `Job` ID immediately, per
  [0023](0023-job-queue.md).
- `UnmatchedFileService` — `Get`/`List` (filtered by status) /`Resolve`
  (accept a candidate, or dismiss) for the review queue.
- An organize-trigger RPC, independent of auto-organize's config state.

### Config

A new pipeline config component (`internal/config`, per
[0010](0010-configuration.md)): scan roots and their content-type mapping,
ignore globs, the confidence threshold (a config default carried forward
from v1 as a *starting* number only — not validated, not a claim it's
correct), the hashing toggles above, the auto-organize toggle, and naming
templates per content type.

## Consequences

- Every content type identification is built on top of (Music first, per
  the agreed sequencing) shares discovery, hashing, grouping-as-a-capability,
  the review queue, the decision service, and the organizer — only
  `FileFingerprinter`/identifier/grouping implementations are written per
  content type.
- `MediaFile` gains one new field (`SHA512`); no other kernel struct
  changes. `UnmatchedFile` is new and entirely pipeline-owned.
- Music's confidence-scoring formula remains genuinely undecided after
  this ADR — intentionally. Anyone picking that work up next reads this
  ADR's Context section first, specifically the "suspiciously convergent
  scores" symptom, rather than re-deriving v1's formula by default.
- Two new persistence categories exist side by side with different
  durability guarantees (`Job` ephemeral, `UnmatchedFile` durable) — worth
  knowing when reasoning about what survives a restart.
- Full-file hashing (`MD5`/`SHA512`) being opt-in means a default
  deployment's file identity rests on `OSHash`+`SHA1` only; enabling the
  stronger hashes is a deliberate operator choice, not silently always-on.

## Self-Audit Checklist

1. Does any code add a pipeline-specific field to `Item`, `MediaFile`
   (beyond the one deliberate `SHA512` addition), `Group`, or
   `LibraryEntry` instead of using their `Metadata` bags? If yes — fix it.
2. Does `FileFingerprinter`, the identifier, or the grouping capability get
   dispatched via a content-type switch statement instead of the
   `ContentTypes()` fan-out registry pattern? If yes — fix it.
3. Does any content-type's identifier decide auto-import vs. queue itself,
   instead of returning a score for the shared decision service to act on?
   If yes — fix it; that split is deliberate.
4. Does a freshly discovered, not-yet-identified file get written as a
   `MediaFile` row (with a fabricated/placeholder `ItemID`) instead of an
   `UnmatchedFile`? If yes — fix it.
5. Does any scan/identify path skip the hash-based "already known" lookup
   before fingerprinting/identifying? If yes — fix it; that's the
   mechanism the Organizer's no-reimport guarantee depends on.
6. Does the Organizer move a file without first updating its
   `MediaFile`/`UnmatchedFile` `Path`, or does auto-organize run with no
   way to invoke organizing manually when the toggle is off? If yes — fix
   it.
7. Is `UnmatchedFile`'s storage forced into the generic single-ID/no-filter
   shape instead of its own translator? If yes — fix it, per
   [0012](0012-datastore-persistence.md) self-audit item 7.
8. Does any code build Music's actual confidence-scoring formula as part of
   implementing this ADR? If yes — that's explicitly out of scope here;
   stop and raise it as its own design pass.
9. Does any long-running operation this ADR introduces track its own
   progress instead of going through [0023](0023-job-queue.md)'s
   `JobPublisher`? If yes — fix it.
