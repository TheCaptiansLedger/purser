# Music `FileFingerprinter` (M4)

Music's implementation of [0024](../adr/0024-pipeline-core.md)'s
`FileFingerprinter` capability. Consumes `GroupingResult.DiscNumber` from
[pipeline-grouping-capability.md](pipeline-grouping-capability.md) (M3) and
produces the consensus `Fingerprint` defined in
[pipeline-unmatchedfile-grouping.md](pipeline-unmatchedfile-grouping.md)
(M1). See [music-pipeline-milestones.md](music-pipeline-milestones.md) for
where M4 sits in the build order.

## Scope

In scope: per-file tag/duration extraction, and reducing a group's
per-file results into the one consensus `Fingerprint` written onto every
`UnmatchedFile` row sharing a `GroupKey`.

Out of scope: deciding what any of this *means* for identification —
candidate generation (M7) and scoring (M8) are the only things allowed to
interpret this data. M4 captures facts, including conflicting ones; it
never resolves them.

## Tooling: `ffprobe`, for both tags and duration

Duration requires reading the actual bitstream, not just tags — a plain
tag-reading library doesn't give you that. `ffprobe` (part of ffmpeg)
returns both embedded format tags and stream duration from one call
(`-show_format -show_streams -of json`), uniformly across FLAC/MP3/M4A/OGG
— one external dependency and one process spawn per file, instead of a
Go tag library plus a separate per-format duration mechanism. This is a
second documented deployment dependency alongside `fpcalc`
([pipeline-music-acoustid-adapter.md](pipeline-music-acoustid-adapter.md),
M5) — both belong in the same "media toolchain" build/deployment note, not
two unrelated footnotes.

Exact tag-key mapping (`ALBUM`/`ALBUMARTIST`/`BARCODE`/etc. as `ffprobe`
actually exposes them per container format — FLAC/OGG Vorbis comments vs.
MP3 ID3v2 frames vs. M4A atoms) needs verification against real files
during implementation. Not asserted solved here — a starting point, same
caveat every other numeric/formula claim in this doc set carries.

## Per-file extraction

For one discovered file, given its path:

1. Run `ffprobe`, parse its JSON output.
2. Raw tags into `Fingerprint.Tags`: `ALBUM`, `ALBUMARTIST`, `DATE`,
   `LABEL`, `CATALOGNUMBER`, `BARCODE`, `ISRC`, `TITLE`,
   `MUSICBRAINZ_ALBUMID`, `MUSICBRAINZ_RELEASEGROUPID`,
   `MUSICBRAINZ_TRACKID`/`MUSICBRAINZ_RELEASETRACKID`.
3. **Disc number resolution** — this is where the `DISCNUMBER` tag
   "overrides the folder guess" promise actually happens:
   ```
   discNumber := groupingResult.DiscNumber // M3's folder-derived guess, 0 if none
   if tag, ok := tags["DISCNUMBER"]; ok {
       if parsed, err := strconv.Atoi(tag); err == nil {
           discNumber = parsed
       }
   }
   ```
   Stored in `Fingerprint.Metadata["disc_number"]`.
4. `TRACKNUMBER` stored as-is into `Fingerprint.Metadata["track_number"]` —
   a raw string, not a parsed int, matching `UnmatchedFile.TrackNumber`'s
   type (M1) and `Item.Sequence`'s existing kernel convention. This is what
   lets vinyl side-lettering (`"A1"`, `"B3"`) survive the pipeline instead
   of being silently dropped by an integer parse; no folder-derived
   fallback exists for track number either way, so there's no parse step
   to fail in the first place.
5. Duration from `ffprobe`'s stream data into
   `Fingerprint.Metadata["duration_seconds"]`.

Each per-file `Fingerprint` as a whole stays **in memory only** — it is
never written wholesale to the `UnmatchedFile` row that's about to be
created for that file; only the group consensus (below) gets persisted
onto `Fingerprint`. **One exception, added by
[pipeline-music-persist.md](pipeline-music-persist.md) (M9):** the
resolved `disc_number`/`track_number` values *also* get written directly
onto that file's own `UnmatchedFile.DiscNumber`/`TrackNumber` fields at
create time, per-row — the group consensus only keeps *ordered lists*
keyed by `(disc, track)`, not which physical file produced each entry, and
persisting (which can happen long after the scan ran) needs to recover
that mapping.

## Group consensus (pass 2, after the task loop)

Per [pipeline-unmatchedfile-grouping.md](pipeline-unmatchedfile-grouping.md)'s
two-pass design: once every task in the job has run pass 1, the buffered
per-file `Fingerprint`s are bucketed by `GroupKey` and reduced into one
consensus `Fingerprint` per group:

- **`Tags`**: majority vote per key across the group's files — a typo'd
  outlier in one file's `ALBUM` tag doesn't win over nine files that agree.
- **`Metadata["track_count"]`**: number of files in the group.
- **`Metadata["disc_count"]`**: highest resolved `disc_number` seen.
- **`Metadata["track_titles"]`, `["track_durations"]`, `["track_isrcs"]`**:
  ordered by `(disc_number int, track_number string)` — the same pair shape
  MusicBrainz's own `medium.position`/`track.number` fields use, so M7/M8
  match position-for-position with no translation needed, vinyl included.
- **`Metadata["embedded_release_mbids"]`**: the **set** of distinct
  non-empty `MUSICBRAINZ_ALBUMID` values seen across the group's files —
  deliberately not collapsed to one value. If two files disagree, that's a
  real conflict scoring needs to see, not something M4 quietly resolves by
  picking one.
- **`Metadata["embedded_releasegroup_mbids"]`**: the same treatment for
  `MUSICBRAINZ_RELEASEGROUPID` — a separate MusicBrainz entity from
  `MUSICBRAINZ_ALBUMID` (release vs. release group), needed as its own
  preserved set because M7's direct-ID short-circuit falls back to this tag
  when the release-ID set doesn't resolve, per
  [pipeline-music-identifier.md](pipeline-music-identifier.md).

The consensus is written onto every `UnmatchedFile` row in the group with
one `UpdateBatch` call — a second, internal caller of the same primitive
M1 built for the UI's `DismissBatch`.

## Dispatch

`FileFingerprinterRegistry`, same shape as M3's `GroupingRegistry`: keyed
by `ContentTypes()`, `ScanExecutor` picks the implementation matching
`job.Params["content_type"]`. Default when nothing's registered:
`NoopFingerprinter`, returning an empty `Fingerprint{}` — a content type
with no fingerprinter yet simply gets no identification signal, same
"no cost to opt out, no crash either" treatment `IdentityGrouping` gets.
