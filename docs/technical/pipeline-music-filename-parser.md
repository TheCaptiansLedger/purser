# Filename/Folder-Name Fallback Parser (M6)

Music's implementation of the generation-only filename fallback described
in [0025](../adr/0025-music-identification-confidence-scoring.md). Feeds
[music-musicbrainz-adapter.md](music-musicbrainz-adapter.md)'s (M2)
`SearchReleaseGroups(ctx, artistName, albumName)`. See
[music-pipeline-milestones.md](music-pipeline-milestones.md) for where M6
sits in the build order.

## Scope

**In scope:** a pure string-parsing capability — given a group's folder
path, produce a best-effort `(artist, album)` guess or no guess at all.

**Explicitly out of scope:**
- *When* to call it. Per [0025](../adr/0025-music-identification-confidence-scoring.md),
  this only runs when tag-derived candidate generation found nothing —
  that's M7's gate, not built here.
- Tagging results as filename-sourced so scoring never trusts them alone.
  M6 returns a raw guess; M7 builds the `MatchCandidate` from whatever the
  search returns and marks its provenance there. M6 never touches
  `MatchCandidate`.
- Parsing individual track filenames as a second-level fallback when the
  folder name itself is unusable. This path is already a last resort whose
  output is never trusted for scoring — "no guess, stays queued" is a safe
  outcome, not a gap worth a second, weaker parsing mechanism.

## No I/O, but the same registry treatment as M3/M4

This is a pure function — no `httpclient`, no `cache`, nothing to reuse
from those packages. It still gets a `ContentTypes()`-registry port, same
shape as `Grouping` (M3) and `FileFingerprinter` (M4), rather than being
special-cased inline: filename/folder-naming conventions are entirely
content-type-specific (music folder naming has nothing in common with
movie filename conventions), so this is the same kind of pluggable
capability, even though Music is the only implementation today.

```
FilenameParser interface {
    ContentTypes() []domain.ContentType
    Parse(ctx context.Context, groupPath, scanRoot string) (artist, album string, ok bool)
}
```

`scanRoot` is the configured root `groupPath` was discovered under. Step 4
below needs to tell "parent directory is a real artist folder" apart from
"parent directory is the scan root itself" — without it, a pure function
given only `groupPath` has no way to make that distinction (there is no
filesystem-level signal that reliably stands in for a caller-configured
root).

Default when nothing's registered for a content type: `ok = false`
always — same "no cost to opt out" treatment `IdentityGrouping` and
`NoopFingerprinter` get.

## Algorithm

Operates on the group's folder path — already at the right level; for a
multi-disc group, `GroupKey` is already the album-level grandparent (M3's
roll-up), so no special-casing needed here.

1. Strip a trailing year annotation from the leaf folder name:
   `"Hi Infidelity (1980)"` → `"Hi Infidelity"`.
2. Reject "junk" names against a small blocklist — `"Music"`,
   `"Downloads"`, `"New Folder"`, purely numeric, too short to be
   meaningful. A starting list, not asserted complete; expected to need
   tuning against real libraries.
3. If the (non-junk) leaf name contains a delimiter (`" - "`, `" – "`,
   `"_-_"`), split on the first occurrence: first part = artist guess,
   second = album guess.
4. Otherwise, if the leaf name alone is non-junk, it's the album guess.
   Look at the *parent* directory name: if it's also non-junk and the
   parent directory itself isn't `scanRoot`, it's the artist guess.
5. A partial result (only one of artist/album populated) is still
   returned with `ok = true` — `SearchReleaseGroups` can search on one
   field alone, just less constrained.
6. If nothing usable survives, `ok = false` — no fabricated guess.

## Where it lives

Same location as M3/M4's implementations — no I/O dependencies means it
could theoretically be a standalone `pkg/` utility, but the actual
heuristics (junk-name blocklist, folder-naming conventions) are
Music-library-specific judgment calls, not generic string processing
worth extracting for reuse outside this module.
