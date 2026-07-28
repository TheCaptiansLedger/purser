# Music Scan Pipeline: Milestone Tracker

Tracks the work needed to bring Music identification online, on top of the
shared pipeline core ([0024](../adr/0024-pipeline-core.md)) and the design
decided in [0025](../adr/0025-music-identification-confidence-scoring.md)
(mechanics in [music-identification.md](music-identification.md)). Update
status as work lands — this doc is state, not a spec; the ADR/technical doc
are the spec.

| ID | Issue | Milestone | Depends on | Status |
|---|---|---|---|---|
| M0 | — | ADR-0025: identification design | — | Done (Proposed; flip to Accepted once M8's fixture suite validates the model) |
| M1a | [#507](https://github.com/TheCaptiansLedger/purser/issues/507) | `Datastore.UpdateBatch` primitive | M0 | Done |
| M1b | [#508](https://github.com/TheCaptiansLedger/purser/issues/508) | `UnmatchedFile` grouping/candidate fields + `ListGroup`/`DismissBatch` | M1a | Done |
| M2 | [#509](https://github.com/TheCaptiansLedger/purser/issues/509) | MusicBrainz adapter | — | Done |
| M3a | [#510](https://github.com/TheCaptiansLedger/purser/issues/510) | Generic `Grouping` port/registry + `ScanExecutor` wiring | M1b | Done |
| M3b | [#511](https://github.com/TheCaptiansLedger/purser/issues/511) | Music's grouping algorithm (folder/multi-disc/multi-LP) | M3a | Done |
| M4 | [#512](https://github.com/TheCaptiansLedger/purser/issues/512) | Music `FileFingerprinter` (ffprobe, vinyl-aware) | M1b | Done |
| M5 | [#513](https://github.com/TheCaptiansLedger/purser/issues/513) | AcoustID adapter | — | Done |
| M6 | [#514](https://github.com/TheCaptiansLedger/purser/issues/514) | Filename/folder-name fallback parser | M2 | Done |
| M7 | [#515](https://github.com/TheCaptiansLedger/purser/issues/515) | Music identifier / candidate generation | M2, M4, M6 | Implemented, pending verification sign-off (Score not yet trusted for auto-import — depends on M8) |
| M8 | [#516](https://github.com/TheCaptiansLedger/purser/issues/516) | Music `ConfidenceScore` + fixture suite | M5, M7 | Implemented, pending verification sign-off (fixture suite + worked-example numbers need manual review before M7's Score is trusted for auto-import) |
| M9-pre | [#532](https://github.com/TheCaptiansLedger/purser/issues/532) | `ExternalIDRepository` `GetByValue` + get-or-create (ADR-0026) | — | Not started — must land before M9b |
| M9a | [#517](https://github.com/TheCaptiansLedger/purser/issues/517) | Shared `DecisionService` (+ adds `config.Pipeline.ConfidenceThreshold`) | M8 | Not started |
| M9b | [#518](https://github.com/TheCaptiansLedger/purser/issues/518) | Music `Persister` cascade + `MusicRelease` reservation fix + `AcceptCandidate` | M9-pre, M9a | Not started |
| M10a | [#519](https://github.com/TheCaptiansLedger/purser/issues/519) | Sidecar classification (registry + Music rules + `Trigger` wiring) | M3a | Implemented, pending manual verification sign-off (see issue checklist) |
| M10b | [#520](https://github.com/TheCaptiansLedger/purser/issues/520) | Cover-art attachment at persist time | M9b | Not started |
| M11a | [#521](https://github.com/TheCaptiansLedger/purser/issues/521) | Generic `Organizer` mechanics (render/move/collision handling) | M9b | Not started |
| M11b | [#522](https://github.com/TheCaptiansLedger/purser/issues/522) | Music `TemplateDataBuilder` + config + trigger wiring | M11a, M9b | Not started |

M2, M5, and M9-pre have no internal dependency and can be built any time
before whatever needs them (M6 for M2, M8 for M5, M9b for M9-pre).

## M0 — ADR-0025: identification design
`docs/adr/0025-music-identification-confidence-scoring.md` +
`docs/technical/music-identification.md`. Decides grouping representation,
candidate generation vs. scoring split, tiered confidence scoring, and the
AcoustID/filename fallback chain.

## M1a/M1b — Pipeline-core plumbing for grouping/candidates
Design: [pipeline-unmatchedfile-grouping.md](pipeline-unmatchedfile-grouping.md).
M1a: a new `Datastore.UpdateBatch` primitive (Badger + SQL). M1b: adds
`GroupKey`/`Fingerprint`/`Candidates` to `domain.UnmatchedFile`;
`UnmatchedFileService.ListGroup` (read) and `DismissBatch` (bulk dismiss,
explicit ID list from the caller — `Resolve`'s single-file match path is
untouched). Generic — not Music-specific — since any future grouping
content type reuses it.

## M2 — MusicBrainz adapter
Design: [music-musicbrainz-adapter.md](music-musicbrainz-adapter.md).
Artist/release-group/release/recording lookup by MBID, barcode lookup,
ISRC lookup, fuzzy release-group search. The join-key provider everything
else depends on. Scoped to serve the scan pipeline only — a manual
"add artist" flow is a noted future consumer of the same adapter, not
part of this effort.

## M3a/M3b — Grouping capability (registry + Music)
Design: [pipeline-grouping-capability.md](pipeline-grouping-capability.md).
M3a: generic `Grouping` port/registry, wired into `ScanExecutor` (runs once
per job, not per file); `config.Pipeline.ScanRoots` gains content-type
pairs. M3b: Music's implementation — folder-based default + multi-disc
subfolder roll-up (`CD1`/`Disc 2` naming patterns). Purely structural — no
tag reading; the `DISCNUMBER` tag override happens later, in M4/M7.

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
That wiring lands alongside M9a/M9b, the first milestones that actually
need real, persisted candidates end-to-end.

## M8 — Music `ConfidenceScore` + fixture suite
Design: [pipeline-music-confidence-score.md](pipeline-music-confidence-score.md).
Batch-shaped port (whole candidate list at once, correcting 0024's
per-candidate framing — same pattern as M3/M4). Per-tier formulas: flat
`direct_id`, base+corroboration `unique_id`, evaluated-signals-only
weighted average `fuzzy` (the actual convergence-bug fix), dual-role
`acoustic` (capped-alone / corroborating-bonus / contradiction-demotion).
Ambiguity cap clamps to a fixed ceiling rather than needing the real
config threshold. Also closes a design-review gap: a coverage floor
generically caps any candidate scored on too little evaluated-signal
weight to the bottom half of its tier's band, so a filename-only
candidate can't reach auto-import territory on a couple of strong signals
alone. Ships with the required fixture suite and an explicit
differentiation assertion.

## M9-pre — `ExternalID` get-or-create (ADR-0026)
Design: [0026](../adr/0026-external-id-get-or-create.md). Adds
`ExternalIDRepository.GetByValue` and turns `Create` into get-or-create on
`(EntityType, Source, Value)`, via the same reservation-document mechanism
0019 built for `Tag`. Not itself Music-specific, but M9b's Artist/Release
Group get-or-create steps call it directly, so it must land first — split
out as its own issue ([#532](https://github.com/TheCaptiansLedger/purser/issues/532))
once M9b's own issue turned out to list this as a bare ADR dependency with
no owning issue.

## M9a/M9b — Decide/persist wiring
Design: [pipeline-music-persist.md](pipeline-music-persist.md). M9a: the
shared, content-type-agnostic `DecisionService` — compares the top
candidate's `Score` against `config.Pipeline.ConfidenceThreshold` (a new
field this milestone adds) and dispatches to the registered `Persister`.
M9b: Music's `Persister` cascade, built on M9-pre's get-or-create mechanism
for Artist/Release Group via `ExternalID`. Two triggers (auto
`DecisionService`, manual `AcceptCandidate` RPC), one `Persister`
capability. VA sentinel resolves itself via ordinary get-or-create, no
separate seed step. `MusicRelease` gets its own Music-local
reservation-document fix (0019's pattern directly, no `ExternalID`
involved). Adds `UnmatchedFile.DiscNumber`/`TrackNumber` (amends M1/M4) and
`DeleteBatch` on `UnmatchedFileRepository` (reuses pre-M1 generic
batch-delete primitives).

## M10a/M10b — Cover art & sidecar classification
Design: [pipeline-music-sidecar-classifier.md](pipeline-music-sidecar-classifier.md).
M10a: classification runs in `ScanService.Trigger`, before Tasks exist —
sidecar files never enter the pipeline at all. Simplifies M3b (its own
"ignore non-audio" filter is now redundant, removed). M10b: cover art isn't
tracked between scan and persist — M9b's `Persist` does a fresh folder
listing at attach time. First real caller of `ImageStore`'s write path (a
gap 0013 already flagged). Duplicate-attachment guard is a plain existence
check, not full reservation-document machinery.

## M11a/M11b — Organizer naming template for Music
Design: [pipeline-music-organizer.md](pipeline-music-organizer.md). M11a: a
generic `Organizer` (render + move, shared), same `ContentTypes()` registry
pattern as M3/M4/M6/M10. `Rename` with a copy-verify-delete fallback for
cross-filesystem moves; refuses to overwrite on a destination collision.
M11b: Music's `TemplateDataBuilder` (cross-entity data gathering: Item +
Group + MusicRelease + LibraryEntry) and the config/trigger wiring.
Per-content-type `{Root, Template}` config, not one global template.
Auto-trigger from M9b's `Persist`; manual trigger via a new
`OrganizerService` RPC.
