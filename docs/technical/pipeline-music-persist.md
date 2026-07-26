# Decide/Persist Wiring (M9)

Turns [pipeline-music-confidence-score.md](pipeline-music-confidence-score.md)
(M8)'s scored candidates into real `Group`/`MusicRelease`/`Item`/
`MediaFile` rows, using [0026](../adr/0026-external-id-get-or-create.md)'s
get-or-create mechanism for Artist/Release Group and a Music-local
equivalent for `MusicRelease`. See
[music-pipeline-milestones.md](music-pipeline-milestones.md) for where M9
sits in the build order.

## Scope

**In scope:** the shared decision service, the manual accept path, Music's
`Persister` capability (the artist → release group → release → item →
media-file cascade), and the reservation-document fix for `MusicRelease`'s
own MBID uniqueness.

**Explicitly out of scope:** cover art / sidecar routing (M10), the
Organizer (M11).

## Two triggers, one `Persister`

- **Automatic**: a shared, generic `DecisionService` ([0024](../adr/0024-pipeline-core.md))
  runs once per group, right after M7/M8 finish scoring. If the top
  candidate's `Score` clears the configured threshold, it calls the
  content-type-dispatched `Persister` for that candidate.
- **Manual**: a new `AcceptCandidate(ctx, groupKey, externalRef string)` RPC.
  `externalRef` is just a raw MusicBrainz release MBID — it does not have
  to be one already sitting in the group's `Candidates` list:
  1. **Matches an existing candidate** — the common case, a human picks
     *any* ranked candidate from the review UI, not necessarily the
     top-scored one. Use that `MatchCandidate` as-is (`Tier`/`Score`/
     `Signals` intact).
  2. **No match** — M7 found nothing (an empty `Candidates` list is a
     normal outcome, not an error), found the wrong releases, or the human
     already knows the correct MBID from MusicBrainz directly and typed it
     in rather than picking from the list. Build an ad-hoc
     `MatchCandidate{ExternalRef: externalRef}` with no `Tier`/`Score` —
     a human-supplied MBID is definitionally corroborated, so it bypasses
     `ConfidenceScore` entirely rather than being scored after the fact.

  Both cases call the exact same `Persister`; the review UI presents the
  ranked list *and* a free-text MBID field, not one or the other.

Both funnel through one capability:

```
Persister interface {
    ContentTypes() []domain.ContentType
    Persist(ctx context.Context, fingerprint *domain.Fingerprint, candidate domain.MatchCandidate, files []*domain.UnmatchedFile) error
}
```

On success, the caller (`DecisionService` or the `AcceptCandidate` handler)
`DeleteBatch`es the group's `UnmatchedFile` rows — matching today's
single-file `Resolve`'s "a matched entry leaves the queue by deletion"
behavior, just applied to the whole group at once.
`ports.UnmatchedFileRepository` gains `DeleteBatch` for this — reusing the
*already-existing* generic batch-delete primitives
(`store.FilteredRepository[T].DeleteBatch` → `Datastore.DeleteBatch`) that
predate M1; no new `Datastore` work needed here, unlike `UpdateBatch`
which M1 had to add from scratch.

On failure, the group's `UnmatchedFile` rows are left untouched (still
`pending`), the error is logged, and — for the manual path — returned to
the caller.

## Music's `Persist`: the cascade

Given the winning candidate (an `ExternalRef` — a MusicBrainz release
MBID), the group's consensus `Fingerprint`, and its `UnmatchedFile` rows:

1. Fetch the release's full data via `LookupRelease` (M2) — this gives the
   parent release group and artist-credit MBIDs, plus the full tracklist.
2. **Artist**: get-or-create via [0026](../adr/0026-external-id-get-or-create.md)'s
   pattern — `GetByValue(library_entry, mbz, artistMBID)`; on a miss,
   speculatively create a `LibraryEntry` from MusicBrainz artist data, then
   link it via `ExternalIDRepository.Create`, reconciling if another
   caller won the race concurrently.
3. **Various Artists sentinel — resolves itself, no separate seed step.**
   [0021](../adr/0021-music-domain-model.md) left *how* the one-time VA
   `LibraryEntry` gets seeded unspecified. It doesn't need a separate
   operational step at all: Various Artists has a real, known MBID like
   any other artist, so the very first VA-credited release persisted just
   runs step 2 above and creates it via ordinary MusicBrainz data — no
   special-cased seeding mechanism required.
4. **Release Group**: same get-or-create pattern as step 2, keyed on
   `(group, mbz, releaseGroupMBID)`, linked to the artist resolved above.
5. **`MusicRelease`**: get-or-create keyed on `MBID` — but `MBID` is a
   *plain field* on `MusicRelease` itself, not routed through `ExternalID`
   (per [0021](../adr/0021-music-domain-model.md)'s explicit decision), so
   this doesn't use 0026's mechanism. It gets its own, Music-local
   reservation-document fix instead — see below. `Status` is set to
   `imported` if every track in the release's official tracklist has a
   corresponding file in the group, `partial` otherwise.
6. **Items + MediaFiles**: for each `UnmatchedFile` in the group, its
   `DiscNumber`/`TrackNumber` (M1/M4's per-row fields — see those docs'
   amendments) locate the matching entry in the release's tracklist. This
   is a direct field match, not a translation: MusicBrainz's own
   medium/track data uses the identical `(medium.position int, track.number
   string)` shape, vinyl included, so a side-lettered `"B3"` matches
   straight across with no parsing on either side, and `Item.Sequence` is
   set directly from `UnmatchedFile.TrackNumber` — both strings, matching
   [0021](../adr/0021-music-domain-model.md)'s original vinyl intent
   exactly. Create an `Item(ContentType=music)` (`Title` from the canonical
   MusicBrainz tracklist, `GroupID` from step 4, `Metadata["release_id"]`
   from step 5, `Sequence`/`RuntimeSeconds`/`ExternalID(mbz_recording)`
   per [0021](../adr/0021-music-domain-model.md)'s Track mapping), then a
   `MediaFile` linking it to that file's path/hashes — the same
   `Create`/`Validate` shape today's `Resolve` already uses for a single
   file, just looped across the group. Tracklist entries with no matching
   file get a stub `Item` (`Status = missing`) rather than being silently
   absent from the release — worth confirming against real MusicBrainz
   data during implementation, not fully speced here.

## `MusicRelease`'s own reservation-document fix

Same mechanism as [0019](../adr/0019-tag-identity-and-get-or-create.md)
directly — no composition wrinkle here, since `MusicRelease` *is* the
entity carrying the unique field (unlike Artist/Release Group's
`ExternalID`-mediated case): a `music_release_mbid` reservation collection,
`CreateBatch`'d alongside the `MusicRelease` document, entirely inside
`internal/adapters/store/music`'s translator — invisible above the
adapter, same as `Tag`'s fix. Scoped to `MBID` only; `Barcode`'s existing
point-lookup stays a plain index, since persist's get-or-create keys on
MBID, not barcode — barcode was already spent as an identification signal
back in M7/M8.

## No true cross-repository transaction — idempotency is the recovery model

Artist, Release Group, `MusicRelease`, and each Item/MediaFile pair are
each written through their own repository's own write — `Datastore` has no
cross-collection transaction primitive spanning all of them
([0012](../adr/0012-datastore-persistence.md)). A partial failure partway
through (say, the 5th of 12 `MediaFile` creates fails) leaves the earlier
steps committed. Recovery relies on every step above being a "look up or
create" — a repeated manual `AcceptCandidate` call finds the
already-created artist/release-group/release via their MBID lookups and
simply continues from wherever it left off, rather than needing rollback.
**Not** "automatic re-decision on next scan": the group's `UnmatchedFile`
rows are still `pending` after a partial failure, and 0024's hash
short-circuit treats any hash match against a pending row as "already
queued," skipping identify/decide entirely — so a future scan will never
retry it on its own. Manual accept is the only recovery path today. This
is the same tradeoff
[0019](../adr/0019-tag-identity-and-get-or-create.md) already accepted for
its own multi-step rename path — extended here, not a new kind of risk.

## Where this runs

Same shape as M7's identify step: once per group, after M8 finishes
scoring — a fourth pass in `ScanExecutor`, after grouping (M3), consensus
fingerprinting (M4), and identify+score (M7/M8). Same job-progress
resolution as M7: the result is recorded on every task's `Step` list in
the group, since `pkg/jobqueue` has no job-level step concept.

## Consequences

- `AcceptCandidate` is a dual-input RPC, not a picker: the review UI must
  offer both the ranked `Candidates` list *and* a free-text MBID field, in
  the same screen — a UI that only renders the ranked list silently drops
  the "I already know the MBID" path this doc exists to support.
- An ad-hoc, human-supplied candidate skips `ConfidenceScore` entirely
  (no `Tier`/`Score` computed for it) — this is intentional, not a
  shortcut that happens to work; a human pasting in an MBID has already
  done the corroboration `ConfidenceScore` exists to approximate.
- `UnmatchedFile.TrackNumber` being a string (not int) means every
  consumer downstream of M4 — M7/M8's `(disc, track)`-keyed signal
  comparisons and this doc's own tracklist-matching in step 6 — compares
  strings, not integers. A future change that reintroduces int parsing
  anywhere in that chain silently breaks vinyl again.
- The "look up or create" recovery story in "No true cross-repository
  transaction" above is only true for Artist/Release Group/`MusicRelease`.
  It is **not yet true for step 6** (Items/MediaFiles) — that step is a
  plain `Create` loop with no hash check, so a retry after a partial
  failure will duplicate the tracks that already succeeded. Flagged here
  as a known gap, not fixed by this doc.

## Self-Audit Checklist

1. Does `AcceptCandidate` reject or otherwise fail to handle an
   `externalRef` that isn't already present in the group's `Candidates`
   list? If yes — fix it; a raw, human-supplied MBID is the whole point of
   this RPC, not an edge case of it.
2. Does an ad-hoc manual candidate (no pre-existing `MatchCandidate`) get
   run through `ConfidenceScore` before being persisted? If yes — fix it;
   that candidate is definitionally corroborated and should bypass scoring.
3. Does the review UI implementation expose only the ranked list, or only
   a free-text MBID field, instead of both at once? If yes — fix it.
4. Does any code parse `UnmatchedFile.TrackNumber` or
   `Fingerprint.Metadata["track_number"]` as an integer, or reject a
   non-numeric value, instead of treating it as an opaque string? If yes —
   fix it; that's exactly the vinyl side-lettering regression this doc's
   type change exists to prevent.
5. Does `DiscNumber` absorb side-lettering (`"A"`/`"B"`) instead of staying
   a plain medium number? If yes — reconsider; side/position belongs on
   `TrackNumber`, not `DiscNumber`.
6. Does step 6 (Items/MediaFiles) get built without a `GetByHash` check
   before `Create`? This is a known, currently-unresolved gap (see
   Consequences) — building step 6 without addressing it ships a
   duplicate-track bug on any partial-failure retry, not a hypothetical.
