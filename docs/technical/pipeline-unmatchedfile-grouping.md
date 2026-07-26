# Pipeline Plumbing: Grouping and Batch Dismiss (M1)

Generic pipeline-core plumbing ([0024](../adr/0024-pipeline-core.md)),
built to unblock Music identification ([0025](../adr/0025-music-identification-confidence-scoring.md))
but not Music-specific — any future content type that groups files reuses
this. See [music-pipeline-milestones.md](music-pipeline-milestones.md) for
where M1 sits in the build order.

## Scope

In scope: the data model and storage/service/API plumbing so a group of
files can share identification data and be bulk-dismissed together.

Explicitly out of scope: turning a `MatchCandidate` into real
`Group`/`MusicRelease`/`Item`/`MediaFile` rows. That's content-type-aware
(Music-specific field mapping) and belongs to M9, not here. M1 only makes
the data exist.

## Domain changes (`internal/domain`)

`UnmatchedFile` gains five fields:

- `GroupKey string` (`validate:"required"`) — files sharing a `GroupKey`
  are one identification unit. Defaults to the file's own `Path` for
  ungrouped content types (a group of one, by construction) — not `ID`:
  grouping runs before a file's `UnmatchedFile.ID` is generated, so `Path`
  is the value actually available at that point. See
  [pipeline-grouping-capability.md](pipeline-grouping-capability.md) (M3).
- `DiscNumber int` and `TrackNumber string` — **per-row**, unlike everything
  else here, which is shared identically across a group. `TrackNumber` is a
  string, not an int, matching `Item.Sequence`'s existing convention
  ([0021](../adr/0021-music-domain-model.md)) — vinyl rips carry
  side-lettered positions (`"A1"`, `"B3"`) in the `TRACKNUMBER` tag, and
  MusicBrainz's own track data (`track.number`) uses the same string shape
  for the same reason; an int field would silently drop that value on
  parse failure. `DiscNumber` stays an int — it identifies which physical
  medium (CD/LP) a file is on, genuinely numeric even in a multi-record
  vinyl box set, since side-lettering is a per-track concern, not a
  medium-numbering one. Added by
  [pipeline-music-persist.md](pipeline-music-persist.md) (M9): the group's
  consensus `Fingerprint` only carries *ordered lists* keyed by
  `(disc, track)`, not which file path produced each entry, so recovering
  "which physical file is this specific track" at persist time — which can
  happen long after the scan that discovered it — needs each row to carry
  its own resolved position, not just contribute to the aggregate. See
  [pipeline-grouping-capability.md](pipeline-grouping-capability.md) (M3).
- `Fingerprint *Fingerprint` — nil until the fingerprinting stage runs.
- `Candidates []MatchCandidate` — nil until the scoring stage runs.

Two new pipeline-owned types, generic across content types:

```
Fingerprint struct {
    Tags     map[string]string // raw extracted tag key/values
    Metadata map[string]any    // computed/derived data (durations, consensus values, acoustic fingerprint)
}

MatchCandidate struct {
    ExternalRef string             // e.g. an MBID
    Title       string             // for review-queue display
    Score       float64            // 0.0–1.0, what the shared decision service compares to the threshold
    Tier        MatchTier
    Signals     map[string]float64 // per-signal breakdown, for the review UI
    Metadata    map[string]any     // content-specific display fields
}

type MatchTier string
const (
    MatchTierDirectID MatchTier = "direct_id"
    MatchTierUniqueID MatchTier = "unique_id"
    MatchTierFuzzy    MatchTier = "fuzzy"
    MatchTierAcoustic MatchTier = "acoustic"
)
```

`Fingerprint`/`Candidates` are open bags, unvalidated field-by-field — same
treatment `MediaFile.Metadata` already gets, since they're computed
internally by the pipeline, never constructed from untrusted client input.

## Storage changes

- `indexOf` in `internal/adapters/store/unmatchedfile` adds
  `"group_key": u.GroupKey` alongside the existing status/hash keys.
- `ports.UnmatchedFileRepository` gains `ListByGroupKey(ctx, groupKey
  string) ([]*domain.UnmatchedFile, error)` — unlike `GetByHash` (wants
  the first match, stops), this needs every row in the group, so it loops
  pages to completion rather than assuming one page covers it.
- **New `Datastore` primitive: `UpdateBatch(ctx, docs []Document) error`**
  — single transaction, all-or-nothing, mirroring `CreateBatch`'s existing
  shape. Implemented in both the Badger and SQL backends, with
  `datastoretest` contract coverage alongside the existing batch methods —
  per [0016](../adr/0016-bulk-operations.md)'s own stated pattern of
  adding batch primitives when a real caller needs one, not up front for
  every entity. `store.FilteredRepository[T]` gains a matching
  `UpdateBatch(ctx, vs []*T) error` wired straight to it, the same way
  `DeleteBatch` is already wired. `ports.UnmatchedFileRepository` gains
  `UpdateBatch(ctx, us []*domain.UnmatchedFile) error` on top of that.

## Service changes (`UnmatchedFileService`)

- `Resolve` is **unchanged** — still matches exactly one file to one
  already-existing `Item`, one row at a time. Linking one file to one item
  never made sense to fan out across a group, so there was never a reason
  to touch it.
- **New: `ListGroup(ctx, groupKey string) ([]*domain.UnmatchedFile,
  error)`** — read-only, wraps `ListByGroupKey`. This is how the review UI
  fetches "every file in this album" to render one card and build the ID
  list for dismissal.
- **New: `DismissBatch(ctx, ids []string) ([]*domain.UnmatchedFile,
  error)`** — takes the exact ID list the caller supplies (the UI already
  has it from `ListGroup`; the server does no `GroupKey` lookup of its
  own), sets `Status = dismissed` on each, and persists all of them in one
  `UpdateBatch` call. All-or-nothing, per [0016](../adr/0016-bulk-operations.md)'s
  default.

## API changes

Two new RPCs on the Connect `UnmatchedFileService`: `ListGroup` (maps to
the service method above) and `DismissBatch` (maps to the service method
above). Both follow [0011](../adr/0011-api-design.md)'s existing
per-entity RPC shape — no new service, no God-endpoint.

## What this deliberately doesn't do

- No group-wide variant of `Resolve`/match — matching stays one file to
  one item, always.
- No "accept a candidate" operation yet — that requires mapping a
  `MatchCandidate` into real domain entities, which is content-type-aware
  and is M9's job.
- No change to how grouping itself is computed (which files get the same
  `GroupKey`) — that's M3, the Music-specific grouping capability. M1 only
  builds the field and the plumbing around it.
