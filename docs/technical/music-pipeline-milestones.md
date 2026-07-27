# Music Scan Pipeline: Milestone Tracker

Tracks the work needed to bring Music identification online, on top of the
shared pipeline core ([0024](../adr/0024-pipeline-core.md)) and the design
decided in [0025](../adr/0025-music-identification-confidence-scoring.md)
(mechanics in [music-identification.md](music-identification.md)). Update
status as work lands — this doc is state, not a spec; the ADR/technical doc
are the spec.

| ID | Milestone | Depends on | Status |
|---|---|---|---|
| M0 | ADR-0025: identification design | — | Done (Proposed; flip to Accepted once M8's fixture suite validates the model) |
| M1 | Pipeline-core plumbing for grouping/candidates | M0 | Not started |
| M2 | MusicBrainz adapter | — | Not started |
| M3 | Music grouping capability | M1 | Done |
| M4 | Music `FileFingerprinter` | M1 | Not started |
| M5 | AcoustID adapter | — | Not started |
| M6 | Filename/folder-name fallback parser | M2 | Done |
| M7 | Music identifier / candidate generation | M2, M4, M6 | Done (Score not yet trusted for auto-import — depends on M8) |
| M8 | Music `ConfidenceScore` + fixture suite | M5, M7 | Not started |
| M9 | Decide/persist wiring | M8 | Not started |
| M10 | Cover art & sidecar classification | M9 | Not started |
| M11 | Organizer naming template for Music | M9 | Not started |

M2 and M5 have no internal dependency and can be built in parallel with M1.

## M0 — ADR-0025: identification design
`docs/adr/0025-music-identification-confidence-scoring.md` +
`docs/technical/music-identification.md`. Decides grouping representation,
candidate generation vs. scoring split, tiered confidence scoring, and the
AcoustID/filename fallback chain.

## M1 — Pipeline-core plumbing for grouping/candidates
Design: [pipeline-unmatchedfile-grouping.md](pipeline-unmatchedfile-grouping.md).
Adds `GroupKey`/`Fingerprint`/`Candidates` to `domain.UnmatchedFile`; a new
`Datastore.UpdateBatch` primitive; `UnmatchedFileService.ListGroup` (read)
and `DismissBatch` (bulk dismiss, explicit ID list from the caller —
`Resolve`'s single-file match path is untouched). Generic — not
Music-specific — since any future grouping content type reuses it.

## M2 — MusicBrainz adapter
Design: [music-musicbrainz-adapter.md](music-musicbrainz-adapter.md).
Artist/release-group/release/recording lookup by MBID, barcode lookup,
ISRC lookup, fuzzy release-group search. The join-key provider everything
else depends on. Scoped to serve the scan pipeline only — a manual
"add artist" flow is a noted future consumer of the same adapter, not
part of this effort.

## M3 — Grouping capability (registry + Music)
Design: [pipeline-grouping-capability.md](pipeline-grouping-capability.md).
Generic `Grouping` port/registry, wired into `ScanExecutor` (runs once per
job, not per file); `config.Pipeline.ScanRoots` gains content-type pairs.
Music's implementation: folder-based default + multi-disc subfolder
roll-up (`CD1`/`Disc 2` naming patterns). Purely structural — no tag
reading; the `DISCNUMBER` tag override happens later, in M4/M7.

## M4 — Music `FileFingerprinter`
Design: [pipeline-music-fingerprinter.md](pipeline-music-fingerprinter.md).
Per-file tag/duration extraction via `ffprobe` (new deployment dependency,
alongside `fpcalc`); `DISCNUMBER` tag overrides M3's folder-derived guess
here. Per-group consensus (majority-voted tags, ordered per-track lists,
conflicting embedded MBIDs preserved as a set, not collapsed) written via
`UpdateBatch` after the task loop, per M1's two-pass design.

## M5 — AcoustID adapter
Design: [pipeline-music-acoustid-adapter.md](pipeline-music-acoustid-adapter.md).
Two separate operations (local `fpcalc` fingerprinting, remote AcoustID
API lookup via `httpclient`+`cache` per M2's pattern), neither run
unconditionally — both gated by M7's "tags weren't enough" decision, not
built here. Response shape kept provider-faithful (nested
recordings/release-groups), not pre-collapsed. Rate limit unverified —
placeholder, flagged tunable.

## M6 — Filename/folder-name fallback parser
Design: [pipeline-music-filename-parser.md](pipeline-music-filename-parser.md).
Pure string-parsing capability (no I/O), folder-name only (leaf +
parent, year-stripping, junk-name blocklist), same `ContentTypes()`
registry pattern as M3/M4. Feeds M2's now-split `SearchReleaseGroups`
free-text method. Generation-only — M6 never touches `MatchCandidate`;
tagging results as filename-sourced is M7's job.

## M7 — Music identifier / candidate generation
Design: [pipeline-music-identifier.md](pipeline-music-identifier.md).
Wires M2 + M4 + M5 + M6 together: direct-ID short-circuit, then candidate
MBIDs collected (not sequential-cascaded) from barcode/ISRC/fuzzy-tag/
filename/AcoustID sources, deduped, each scored against every applicable
signal. AcoustID's trigger is structural (no unique-ID match found), not
score-based — keeps M7 independent of M8. Sets `Tier`/`Signals`/
`Metadata` only; never sets `Score` — that boundary belongs to M8.
Group-level work recorded redundantly across every task's Step in the
group, since `pkg/jobqueue` has no job-level step concept.

Shipped as a standalone `ports.Identifier`/`Identifier` capability plus
fixture-backed tests, matching #515's own Scope/Verification checklist —
**not yet wired into `ScanExecutor` or `cmd/purser`'s composition root**
(no `MusicBrainzClient`/`AcoustIDClient` are constructed there yet, and
`ScanService.Trigger` doesn't thread `scan_root` through `job.Params`).
That wiring lands alongside M9, the first milestone that actually needs
real, persisted candidates end-to-end.

## M8 — Music `ConfidenceScore` + fixture suite
Design: [pipeline-music-confidence-score.md](pipeline-music-confidence-score.md).
Batch-shaped port (whole candidate list at once, correcting 0024's
per-candidate framing — same pattern as M3/M4). Per-tier formulas: flat
`direct_id`, base+corroboration `unique_id`, evaluated-signals-only
weighted average `fuzzy` (the actual convergence-bug fix), dual-role
`acoustic` (capped-alone / corroborating-bonus / contradiction-demotion).
Ambiguity cap clamps to a fixed ceiling rather than needing the real
config threshold. Ships with the required fixture suite and an explicit
differentiation assertion.

## M9 — Decide/persist wiring
Design: [pipeline-music-persist.md](pipeline-music-persist.md). Depends on
[0026](../adr/0026-external-id-get-or-create.md) (new ADR — get-or-create
for Artist/Release Group via `ExternalID`, extending 0019's pattern).
Two triggers (auto `DecisionService`, manual `AcceptCandidate` RPC), one
`Persister` capability. VA sentinel resolves itself via ordinary
get-or-create, no separate seed step. `MusicRelease` gets its own
Music-local reservation-document fix (0019's pattern directly, no
`ExternalID` involved). Adds `UnmatchedFile.DiscNumber`/`TrackNumber`
(amends M1/M4) and `DeleteBatch` on `UnmatchedFileRepository` (reuses
pre-M1 generic batch-delete primitives).

## M10 — Cover art & sidecar classification
Design: [pipeline-music-sidecar-classifier.md](pipeline-music-sidecar-classifier.md).
Classification runs in `ScanService.Trigger`, before Tasks exist — sidecar
files never enter the pipeline at all. Simplifies M3 (its own "ignore
non-audio" filter is now redundant, removed). Cover art isn't tracked
between scan and persist — M9's `Persist` does a fresh folder listing at
attach time. First real caller of `ImageStore`'s write path (a gap 0013
already flagged). Duplicate-attachment guard is a plain existence check,
not full reservation-document machinery.

## M11 — Organizer naming template for Music
Design: [pipeline-music-organizer.md](pipeline-music-organizer.md). Split
into a generic `Organizer` (render + move, shared) and Music's
`TemplateDataBuilder` (cross-entity data gathering: Item + Group +
MusicRelease + LibraryEntry), same `ContentTypes()` registry pattern as
M3/M4/M6/M10. `Rename` with a copy-verify-delete fallback for
cross-filesystem moves; refuses to overwrite on a destination collision.
Per-content-type `{Root, Template}` config, not one global template.
Auto-trigger from M9's `Persist`; manual trigger via a new
`OrganizerService` RPC.
