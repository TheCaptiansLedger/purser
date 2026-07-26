# Music Identifier / Candidate Generation (M7)

Wires [music-musicbrainz-adapter.md](music-musicbrainz-adapter.md) (M2),
[pipeline-music-fingerprinter.md](pipeline-music-fingerprinter.md) (M4)'s
consensus `Fingerprint`, [pipeline-music-acoustid-adapter.md](pipeline-music-acoustid-adapter.md)
(M5), and [pipeline-music-filename-parser.md](pipeline-music-filename-parser.md)
(M6) together into the cascade summarized in
[music-identification.md](music-identification.md). See
[music-pipeline-milestones.md](music-pipeline-milestones.md) for where M7
sits in the build order.

## Scope

**In scope:** given a group's consensus `Fingerprint`, produce a
deduplicated `[]domain.MatchCandidate` with `Tier`, `Signals`, and
`Metadata` populated for each.

**Explicitly out of scope:** computing a final `Score`. M7 sets `Tier`
(structural — which evidence class applies) and `Signals` (raw per-signal
measurements); M8 owns turning those into a number and handling
cross-candidate ambiguity. **M7 never sets `Score`.** This is a hard
boundary, not a suggestion — see "The M7/M8 boundary" below.

## Where this runs, and the job-progress gap it exposed

Same shape as M4's consensus pass: identification happens once per group,
after M4's consensus `Fingerprint` has been written. But `pkg/jobqueue`'s
`Step`s attach to a `taskID`, and a `Task` is one file — group-level work
doesn't have a natural task to attach to. Redesigning `pkg/jobqueue` for
job-level steps is out of scope for this milestone, so the resolution is
the same one M4 uses: run the identification work once per group, then
record the identical result on every task's `Step` list in that group.
Redundant records, but it means per-file progress polling still reports
accurately for every file, with zero `pkg/jobqueue` schema changes.

## The cascade

Given a group's consensus `Fingerprint`:

### 1. Direct-ID short-circuit

`MUSICBRAINZ_ALBUMID` is conventionally a **Release** MBID;
`MUSICBRAINZ_RELEASEGROUPID` is a separate **Release Group** MBID —
different MusicBrainz entities, standard tagger convention (Picard writes
both). Check, in order:

1. `Fingerprint.Metadata["embedded_release_mbids"]` (the set M4 preserves,
   not collapsed) — if it contains exactly one value, `LookupRelease` it.
2. If that's empty or didn't resolve, try `MUSICBRAINZ_RELEASEGROUPID` via
   `LookupReleaseGroup` the same way.

A conflicting set (more than one distinct embedded ID across the group's
files) skips this step entirely — neither value is trusted. A resolving
lookup still needs a cheap sanity check (the group's `track_count`
roughly matches the resolved release's track count) before it's accepted.
On success: return **one** candidate, `Tier = direct_id`, and stop — every
other step below is skipped, per [0025](../adr/0025-music-identification-confidence-scoring.md).

### 2. Collect candidate MBIDs (not a sequential cascade)

If step 1 didn't resolve, gather candidate release MBIDs from every
applicable source **together**, not "try one, fall back to the next":

- Barcode search (`Fingerprint.Tags["BARCODE"]`, if present).
- ISRC-consensus search (`Fingerprint.Metadata["track_isrcs"]` — look up
  each non-empty ISRC, keep whichever release most of them agree on).
- Fuzzy tag search (`SearchReleaseGroups(ctx, albumArtist, album)` from
  `ALBUMARTIST`/`ALBUM`, then resolve each result release group to its
  specific releases).
- Filename-seeded fuzzy search — **only if the fuzzy tag search above
  returned zero candidates** (empty tags, or tags so wrong the search
  came back empty either way). Calls M6's parser, then the same
  `SearchReleaseGroups` method with the parsed strings.
- AcoustID — **only if nothing above produced a barcode/ISRC (unique-ID)
  match.** This gate is structural (did we find a unique identifier or
  not), never score-based — M7 has no score to check yet, and doesn't
  need one to make this call. Runs `Fingerprint`+`Lookup` for every file
  in the group, not a sample: this path is already the less-common case,
  and it's often the only evidence available, so completeness matters
  more here than the extra cost.

All discovered MBIDs are unioned into one deduplicated set — the same
release can plausibly surface from more than one source (a barcode hit
and a fuzzy name match landing on the same release is common, not an
edge case).

### 3. Score every distinct candidate against every applicable signal

For each distinct MBID in the deduplicated set: fetch its full release
data once (`LookupRelease`), then compute every signal that applies to it
**regardless of which source originally surfaced it** — barcode match,
ISRC consensus, name-fuzzy score, track-count match, title-set overlap,
duration match, AcoustID agreement (if AcoustID ran). A release found via
fuzzy name search whose barcode also happens to match gets `Tier =
unique_id`, not stuck at `fuzzy` because of how it was first found — tier
is assigned from the strongest evidence that actually applies, not the
discovery path.

### 4. Provenance, not a score cap

`Metadata["sources"]` records which discovery method(s) surfaced each
candidate (`"barcode"`, `"isrc"`, `"tag_fuzzy"`, `"filename"`,
`"acoustic"` — a candidate can have several). This is informational for
the review UI, not a scoring input. **Filename-sourced candidates are not
capped.** Nothing about `sources` containing `"filename"` changes the
signal math in step 3 — a bad filename guess simply won't have real
track-count/title-set/duration corroboration and will score low on its
own merits, the same way a bad fuzzy-tag guess would.

## The M7/M8 boundary

| | M7 | M8 |
|---|---|---|
| `Tier` | Sets it (structural fact) | Uses it to pick which scoring band/formula applies |
| `Signals` | Populates raw per-signal values | Combines them into a number |
| `Score` | Never sets it | Computes it |
| Ambiguity across candidates | N/A — M7 doesn't compare candidates to each other | Owns the release-group ambiguity cap |

This split resolves what would otherwise be a circular dependency: "should
we bother calling AcoustID" sounds like it needs a score, but M8 (which
owns scoring) depends on M7, not the reverse. Making the AcoustID trigger
structural (evidence class found, not score achieved) keeps M7 fully
self-contained — it can run, and produce a complete, correctly-tiered
candidate list, with zero knowledge of how M8's formula works.

## Empty result

If every step above finds nothing (no direct ID, no barcode/ISRC/fuzzy/
filename/acoustic candidates at all), M7 returns an empty
`[]MatchCandidate`. No special-casing needed downstream — M9's shared
decision service has nothing to compare against the threshold, so the
group simply stays in the review queue with an empty candidate list, the
same outcome as a low-scoring candidate would produce.
