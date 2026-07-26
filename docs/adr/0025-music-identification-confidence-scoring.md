# 0025. Music Identification: Grouping and Confidence Scoring

Status: Proposed

## Context

[0024](0024-pipeline-core.md) deferred Music's `FileFingerprinter`,
grouping, identifier, and `ConfidenceScore`. [0021](0021-music-domain-model.md)
deferred the same territory from the domain side. This ADR is that pass.

v1 built this twice and both attempts failed the same way: well-tagged
files still didn't produce confident, *differentiated* scores — candidates
converged on similar numbers regardless of actual match quality. The likely
cause was normalizing a candidate's score against the weight of every
*possible* signal instead of just the signals actually evaluable for that
file, plus scoring off boolean fired/not-fired signals instead of
continuous ones. This ADR's design exists specifically to make that failure
structurally impossible, not to retune v1's weights.

Mechanics, worked examples, and the exact signal math live in
[docs/technical/music-identification.md](../technical/music-identification.md) —
this document records the decisions only.

## Decision

1. **Candidate generation and scoring are separate steps.** Tags feed both;
   filename/folder-name parsing is a candidate-generation fallback only,
   used solely when tag-derived generation finds nothing, and is never
   itself scoring evidence. A filename-only candidate is never eligible for
   auto-import.

2. **`UnmatchedFile` gains a generic `GroupKey` field**, not a
   Music-specific group type (rejecting v1's separate `MusicScanGroup`).
   Files sharing a `GroupKey` are one identification unit; resolving one
   resolves all of them. Two new pipeline-owned, generic-across-content-
   types entities are added alongside it: `Fingerprint` (extracted tags +
   an open metadata bag) and `MatchCandidate` (score, tier, per-signal
   breakdown, open metadata bag).

3. **Scoring is tiered** (`direct_id` \| `unique_id` \| `fuzzy` \|
   `acoustic`), not one flat weighted sum. Tiers sit in clearly separated
   score bands so different classes of evidence can't land on the same
   number. Within a tier, every signal is evaluated as a coverage fraction
   across *all* files in the group, normalized only against signals that
   were actually evaluable — never against every file individually, and
   never against signals that had no data to check.

4. **A close runner-up caps the winning score**, computed inside Music's
   own `ConfidenceScore`. The shared decision service only ever sees one
   number per group, so ambiguity between two release-group-level
   candidates has to be folded in before that number is returned.
   Ambiguity between editions/pressings of the *same* release group is not
   treated this way — see the technical doc for why.

5. **Multi-disc folders roll up to one group.** Folder-naming patterns
   (`CD1`, `Disc 2`, etc.) are the primary signal; embedded
   `DISCNUMBER`/`DISCTOTAL` tags override the folder guess when present.
   When neither is conclusive, subfolders are left as separate groups
   rather than guessed together — a wrong split is recoverable at review,
   a wrong merge is not.

6. **AcoustID gets a real adapter**: decode → Chromaprint (`fpcalc`) →
   AcoustID API → resolve MBIDs. It runs only when tag-derived signals
   don't already resolve the group, and it can demote a tag-derived
   candidate it contradicts, not just add to one it agrees with.

## Consequences

- `UnmatchedFile`'s storage translator, API convert layer, and
  `Resolve` semantics all change to operate per-`GroupKey`, not per-file —
  this ripples into existing k6 fixtures.
- `fpcalc`/Chromaprint becomes a runtime dependency wherever AcoustID
  matching is enabled; must be documented alongside build/deployment docs.
- `Fingerprint`/`MatchCandidate` are reusable by any future content type
  that needs multi-signal identification, not Music-only types.
- The score bands in the technical doc are a starting proposal to validate
  against real fixtures, same caveat [0024](0024-pipeline-core.md) already
  gave for the confidence threshold — not asserted correct on first pass.

## Self-Audit Checklist

1. Does any candidate's score get normalized against the weight of signals
   that had no data to evaluate, instead of only the signals actually
   checked? If yes — fix it; that's the mechanism believed to cause v1's
   convergence bug.
2. Does any signal get decided from a single file in a group instead of a
   coverage fraction across every file in it? If yes — fix it.
3. Does a filename-sourced candidate ever score above the auto-import
   threshold without independent corroboration? If yes — fix it.
4. Does `ConfidenceScore` return a top score without checking for a close
   release-group-level runner-up? If yes — fix it.
5. Does any code introduce a Music-specific group type instead of using
   `UnmatchedFile.GroupKey`? If yes — fix it.
6. Does multi-disc grouping ever merge ambiguous subfolders instead of
   leaving them split? If yes — fix it.
7. Does grouping, fingerprinting, or scoring dispatch via a content-type
   switch statement instead of [0024](0024-pipeline-core.md)'s
   `ContentTypes()` registry? If yes — fix it.
