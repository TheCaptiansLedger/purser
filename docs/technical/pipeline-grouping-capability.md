# Pipeline Plumbing: the Grouping Capability (M3)

Generic registry/wiring ([0024](../adr/0024-pipeline-core.md)) plus
Music's concrete implementation ([0025](../adr/0025-music-identification-confidence-scoring.md)).
Builds on `UnmatchedFile.GroupKey` from
[pipeline-unmatchedfile-grouping.md](pipeline-unmatchedfile-grouping.md)
(M1). See [music-pipeline-milestones.md](music-pipeline-milestones.md) for
where M3 sits in the build order.

## Scope

In scope: the `Grouping` port and registry (generic), wiring it into the
scan pipeline, and Music's folder/multi-disc grouping implementation.

Out of scope: anything that reads embedded file tags. Grouping is purely
structural (paths and directory names only) — no dependency on M4
(`FileFingerprinter`). See "What grouping deliberately doesn't do" below.

## Config change

`config.Pipeline.ScanRoots` changes from `[]string` to a list of
`{Path string; ContentType domain.ContentType}` pairs — nothing in the
pipeline can pick a content-type-specific capability without knowing a
root's content type, and today's flat string list doesn't carry it.

## Wiring: grouping runs once per Job, inside `ScanExecutor`

`ScanService.Trigger(ctx, root)` resolves `root`'s `ContentType` (matching
against the configured `ScanRoots`, longest-prefix match so an
on-demand/watcher-triggered scan of a subfolder still resolves correctly)
and adds it to the existing `Job.Params` map — the same mechanism already
carrying `enable_md5`/`enable_sha512`, so no `pkg/jobqueue` schema change.

`ScanExecutor.Execute` reads `job.Params["content_type"]` once, looks up
the matching `Grouping` implementation (falling back to `IdentityGrouping`
if none is registered for that content type), and calls it **once for the
whole job** with every task's path — not once per file. The multi-disc
roll-up needs to see a folder's siblings to know whether they're *all*
disc-named, which a single path in isolation can't answer; the executor
already has every task's label before it starts iterating, so this costs
nothing extra to compute up front. The resulting `path → GroupingResult`
map is consulted per task inside the existing `runQueueStep`, setting
`UnmatchedFile.GroupKey` before `Create`; the `DiscNumber` half is handed
to M4's fingerprinting step (see below), not written anywhere itself.

## The `Grouping` port

```
GroupingResult struct {
    GroupKey   string
    DiscNumber int // 0 if not part of a detected multi-disc structure
}

Grouping interface {
    ContentTypes() []domain.ContentType
    GroupKeys(ctx context.Context, paths []string) (map[string]GroupingResult, error)
}
```

Batch-shaped deliberately — `paths` is every file discovered in one scan
job, `GroupKeys` returns each path's result in one call. A
`GroupingRegistry` holds the registered implementations keyed by
`ContentTypes()` and dispatches `ScanExecutor`'s single call to the right
one, falling back to `IdentityGrouping`.

`IdentityGrouping` (the default): every path maps to `{GroupKey: itself,
DiscNumber: 0}` — one file, one group, no roll-up. This is the corrected
default from M1: **`Path`, not `ID`** — grouping runs before a file's
`UnmatchedFile.ID` exists.

**`DiscNumber` is a guess, not a decision.** It's the disc-subfolder
roll-up's structural inference — carried forward so M4 (`FileFingerprinter`)
has something to start from, since a file's embedded `DISCNUMBER` tag
always wins once it's actually read. See
[pipeline-music-fingerprinter.md](pipeline-music-fingerprinter.md) (M4)
for where it's consumed and overridden.

## Music's grouping algorithm

For each discovered path:

1. Take the immediate parent directory.
2. If that directory's name matches a disc pattern (`CD1`, `CD 1`,
   `Disc1`, `Disc 2`, `D3`, `LP1`, `LP 2`, case-insensitive, digit suffix —
   `LP#` covers multi-record vinyl box sets the same way `CD#` covers
   multi-disc CD sets; a single LP's own A/B sides are a per-track
   concern, handled by `TrackNumber` being a string, not a folder-level
   pattern) **and** every
   sibling directory under its parent that contains a discovered audio
   file also matches the pattern, `GroupKey` = the grandparent directory —
   every file across every disc subfolder shares one group — and
   `DiscNumber` = the digit extracted from the matched pattern (e.g.
   `CD2` → `2`).
3. Otherwise, `GroupKey` = the immediate parent directory (the ordinary,
   non-multi-disc case).
4. If step 2's pattern match is inconsistent (some siblings match, some
   don't) or ambiguous, don't roll up — each subfolder stays its own
   group. A wrong split is recoverable at review; a wrong merge isn't.

**Correction from M10:** grouping used to need its own "ignore non-audio
files" filter here, so a stray `cover.jpg` sitting next to `CD1`/`CD2`
wouldn't break the sibling-pattern check. That's now redundant —
[pipeline-music-sidecar-classifier.md](pipeline-music-sidecar-classifier.md)
(M10) classifies and excludes non-audio files from the task list entirely,
in `ScanService.Trigger`, *before* grouping ever runs. Grouping never sees
a non-audio path in the first place, so it doesn't need its own copy of
the same extension check — one authoritative classification, not two
places that could drift apart.

## What grouping deliberately doesn't do

- **No tag reading.** The `DISCNUMBER`/`DISCTOTAL` tag "overriding the
  folder guess" (from the identification design discussion) is about each
  file's disc-*number value* once fingerprinting reads real tags — it
  never changes which `GroupKey` a file has. Grouping (M3) only produces
  the folder-inferred guess; tag data can only override that value later,
  in M4/M7, not the grouping decision itself.
- **No sidecar routing.** That's [pipeline-music-sidecar-classifier.md](pipeline-music-sidecar-classifier.md)
  (M10)'s job entirely — grouping doesn't classify files, it just no
  longer has to defend against non-audio ones reaching it at all.
- **No tag-based group merging.** An earlier idea — merging folders whose
  audio files share consensus `ALBUM`/`ALBUMARTIST`/MusicBrainz-ID tags
  even when folder names don't match a disc pattern — was considered and
  deliberately left out of M3's scope to avoid both the tag dependency
  above and unbounded scope growth. Flagged here in case it's worth
  revisiting once real-world folder-naming variance shows up in practice.
