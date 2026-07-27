# Music Identification: Mechanics and Worked Examples

Decisions live in [ADR-0025](../adr/0025-music-identification-confidence-scoring.md).
This doc is the how, including two full worked examples. Numbers below
(score bands, thresholds) are starting proposals to validate against real
fixtures, not final.

## Pipeline shape

```
discover ──▶ hash ──▶ sidecar classify ──▶ group ──▶ fingerprint ──▶ generate candidates ──▶ score ──▶ decide
```

Everything through `sidecar classify` is shared pipeline core
([0024](../adr/0024-pipeline-core.md)). Grouping through scoring is Music-owned.
`decide` is shared again — it only ever sees one `Score` per group.

## Grouping

Default: one folder = one group (`GroupKey` = the folder path).

Multi-disc override: if a folder's only children (ignoring sidecar files)
are subfolders matching a disc-naming pattern (`CD1`, `CD 1`, `Disc1`,
`Disc 1`, `D1`, `LP1`, `LP 2`, case-insensitive — `LP#` for multi-record
vinyl box sets), the group rolls up to that parent
folder — every file across every disc subfolder shares one `GroupKey`.
Embedded `DISCNUMBER`/`DISCTOTAL` tags, once read during fingerprinting,
override the folder-derived disc number if they disagree. If subfolder
names don't match a known pattern and tags don't settle it, the subfolders
stay separate groups.

Once grouped, a track's identity for every downstream comparison is the
pair `(DiscNumber, TrackNumber)`, not `TrackNumber` alone — otherwise CD1
track 1 and CD2 track 1 collide.

## Fingerprint: per-group consensus, not per-file

The group's `Fingerprint.Tags` is the majority value across every file in
the group for each tag (`ALBUM`, `ALBUMARTIST`, `BARCODE`, etc.) — not
whichever file the scanner happened to read first. Track titles, per-track
durations (read from the bitstream), and per-track ISRCs are kept as
ordered lists keyed by `(DiscNumber, TrackNumber)`.

## Candidate generation

Full algorithm: [pipeline-music-identifier.md](pipeline-music-identifier.md)
(M7). Summary: an embedded `MUSICBRAINZ_ALBUMID`/`RELEASEGROUPID` tag that
resolves cleanly short-circuits everything else as a `direct_id` candidate.
Otherwise, candidate release MBIDs are *collected* from barcode search,
ISRC-consensus search, and fuzzy `ALBUMARTIST`/`ALBUM` search together
(not a strict "try one, then the next" cascade), deduped by MBID, and each
distinct candidate gets scored against every signal that applies to it —
so a release found via fuzzy name search whose barcode also matches gets
promoted, not stuck at the tier it was first found under. The
filename/folder-name parser only seeds a search when the tag-based fuzzy
search found *zero* candidates, and AcoustID only runs when nothing above
found a unique-ID (barcode/ISRC) match — both gates are structural, not
score-based. Filename-sourced candidates carry a provenance tag but are
**not** score-capped — a bad guess simply won't have real corroborating
signals and will score low on its own.

## Scoring

Full formulas, the signal-key contract with M7, and the fixture suite:
[pipeline-music-confidence-score.md](pipeline-music-confidence-score.md)
(M8). Summary:

| Tier | Trigger | Band | 
|---|---|---|
| `direct_id` | Embedded MB ID resolves, and the group's file/track count is roughly sane for that release | ~0.98 |
| `unique_id` | Barcode or ISRC consensus reached across most files in the group | 0.80–0.95 |
| `fuzzy` | Two or more of {name fuzzy match, track count, title-set overlap, duration} agree | 0.45–0.75 |
| `acoustic` | AcoustID is the only or contradicting evidence | ≤0.60 unless corroborated |

Every `fuzzy` signal is a **coverage fraction across the group**, e.g.
"11 of 12 track titles matched," not a single file's yes/no. The weighted
average is taken over signals that had data to evaluate — a group with no
barcode tag simply doesn't include a barcode term, it isn't scored as if
barcode failed to match.

**Release-group ambiguity vs. edition ambiguity are handled differently.**
If two *release groups* (different albums) both plausibly match, and their
scores are close, the top score is capped below the auto-import threshold
and both go to review — this is genuine ambiguity a human should resolve.
If the release group is clear but multiple *editions/pressings* of it
score similarly (common — track lists and durations rarely differ between
a release group's editions), the pipeline resolves to that release group's
`IsDefault` `MusicRelease` rather than blocking. Most users care which
album matched, not which specific pressing; the edition can be corrected
later the same way any other field is.

## Worked example 1: "Hi Infidelity" — REO Speedwagon (single disc)

Folder: `/music/REO Speedwagon/Hi Infidelity (1980)/`, 10 FLAC files.
Tags present: `ALBUMARTIST=REO Speedwagon`, `ALBUM=Hi Infidelity`,
`TRACKNUMBER`/`TRACKTOTAL=10`, per-track `TITLE`. No barcode, no ISRC, no
embedded MusicBrainz ID (a plain rip, never touched by Picard).

1. **Discover/hash**: 10 files hashed; none match an existing `MediaFile`
   or pending `UnmatchedFile` — genuinely new.
2. **Group**: no disc-subfolder pattern → one group, `GroupKey` = the album
   folder path.
3. **Fingerprint**: consensus tags agree across all 10 files (no
   conflicting outliers). Durations read from each file.
4. **Generate candidates**: no MB ID, no barcode/ISRC → fuzzy search
   `"REO Speedwagon" "Hi Infidelity"` against MusicBrainz. Returns one
   release group, with two known editions (1980 original, 2004 reissue).
5. **Score**: against the release group — name-fuzzy ~1.0, track count
   10/10, title-set 10/10 match, duration 9/10 tracks within tolerance
   (one track runs a few seconds long on this rip) → `fuzzy` tier, near
   the top of its band since every evaluable signal strongly agrees.
   Against the two candidate *editions* — both score nearly identically,
   since track list and durations don't distinguish a 1980 pressing from a
   2004 reissue without a barcode. Per the edition-ambiguity rule above,
   this doesn't trigger the ambiguity cap — the release group is
   unambiguous, so the group's `IsDefault` edition is selected.
6. **Decide**: score clears the auto-import threshold. `Group` "Hi
   Infidelity" and its default `MusicRelease` are created/linked, 10
   `Item`/`MediaFile` rows created, all 10 `UnmatchedFile` rows (same
   `GroupKey`) marked `matched`.

## Worked example 2: "Enhanced" box set — Stevie Nicks (3 discs)

Illustrative box set, folder structure:

```
/music/Stevie Nicks/Enhanced [Box Set]/
  CD1/  (12 files)
  CD2/  (12 files)
  CD3/  (8 files)
  cover.jpg
```

1. **Discover/hash**: 32 audio files + 1 image found.
2. **Sidecar classify**: `cover.jpg` is pulled out before identification —
   it isn't scored as an unmatched track; it's held to become an `Image`
   once a `MusicRelease` resolves.
3. **Group**: the top folder's only children (excluding the sidecar) are
   `CD1`/`CD2`/`CD3`, all matching the disc pattern → rolls up to one
   group at the "Enhanced [Box Set]" level. All 32 files share one
   `GroupKey`.
4. **Disc numbering**: folder-derived `DiscNumber` (1/2/3) matches each
   file's own `DISCNUMBER`/`DISCTOTAL=3` tag — no conflict, tags simply
   confirm the folder guess here.
5. **Fingerprint**: consensus `ALBUMARTIST=Stevie Nicks`,
   `ALBUM=Enhanced`. Track titles/durations recorded per `(Disc, Track)`.
6. **Generate candidates**: fuzzy search on "Stevie Nicks" + "Enhanced"
   returns two release groups from MusicBrainz — the real 3-disc box set,
   and an unrelated single-disc compilation whose title also fuzzy-matches
   "Enhanced." Name-fuzzy alone can't tell them apart.
7. **Score**: track-count/medium-count signals decide it — the real box
   set candidate has `MediumCount=3`, `TrackCount=32`, matching exactly;
   the false-positive candidate has `MediumCount=1`, `TrackCount=11`. That
   gap is large enough that the two candidates' scores aren't close, so no
   ambiguity cap is needed even though the name-fuzzy signal alone was
   ambiguous. Title-set comes back at ~30/32 clean matches — two tracks
   differ only in minor text formatting (e.g. "Edge of Seventeen (Live)"
   vs. "Edge Of 17"), which fuzzy title comparison still counts as
   probable matches at lower confidence rather than "missing." No
   barcode/ISRC tags exist on this rip either, so AcoustID's structural
   gate (no unique-ID match found) still fires and it runs for all 32
   files — that gate is purely "did we find a unique identifier," never a
   confidence check, per
   [pipeline-music-identifier.md](pipeline-music-identifier.md)'s M7/M8
   boundary (M7 has no score to check yet). It just turns out not to be
   *needed* here: the structural disc/track-count signals already
   separate the two candidates decisively on their own, so AcoustID's
   agreement, once it comes back, corroborates the winner without being
   what actually decided it.
8. **Decide**: score clears the auto-import threshold. All 32 tracks are
   present and positionally matched, so the resulting `MusicRelease.Status`
   is `imported`, not `partial`. The two title-text mismatches are kept in
   the candidate's `Signals` breakdown even after import, so the review UI
   can still show why the title-set score wasn't a clean 1.0 if a user
   looks later.

## Open items not resolved by this doc

- Exact score-band boundaries and the auto-import threshold need real
  fixtures before being trusted — see ADR-0025's Consequences.
- Fuzzy string-distance algorithm choice (e.g. Jaro-Winkler vs. token-set
  ratio) is an implementation detail, not pinned here.
- Chromaprint/`fpcalc` packaging for the build/release pipeline is tracked
  separately, not decided in this doc.
