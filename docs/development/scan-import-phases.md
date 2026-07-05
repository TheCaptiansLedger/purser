# Scan / Import Pipeline Fix — Phases 2–10

Implementation plan approved 2026-07-04. Phase 1 (ImportStudio → ImportEntry rename) is done. This document covers phases 2–10 in detail.

Each phase is independently committable and should be committed with the `Part-Of: #360` footer.

---

## Phase 2 — AcoustID response cache

**Bug:** Every call to `Identify()` on a music file makes a live HTTP request to `api.acoustid.org`. AcoustID responses for a given fingerprint never change, so repeated scans of the same files burn quota and add latency. Every other external adapter already uses `pkg/cache`; `acoustid_client.go` is the only gap.

**Files to change:**

### `internal/adapters/identifier/acoustid_client.go`

Add `*cache.Cache` to `acoustidClient`. Key format: `fingerprint + ":" + strconv.Itoa(durationSecs)`. TTL: 7 days (AcoustID recording-to-fingerprint mappings are stable; longer TTL than the 24h used by source adapters).

```go
// Before
type acoustidClient struct {
    apiKey  string
    baseURL string
    http    *http.Client
}

func newAcoustIDClient(apiKey string) *acoustidClient {
    return &acoustidClient{apiKey: apiKey, baseURL: acoustidBaseURL, http: &http.Client{}}
}
```

```go
// After
const acoustidCacheTTL = 7 * 24 * time.Hour

type acoustidClient struct {
    apiKey  string
    baseURL string
    http    *http.Client
    cache   *cache.Cache  // nil = caching disabled (tests)
}

func newAcoustIDClient(apiKey string, c *cache.Cache) *acoustidClient {
    return &acoustidClient{apiKey: apiKey, baseURL: acoustidBaseURL, http: &http.Client{}, cache: c}
}
```

In `Lookup()`, add a cache get/set around the HTTP call:

```go
func (c *acoustidClient) Lookup(ctx context.Context, fingerprint string, durationSecs int) ([]string, error) {
    cacheKey := fingerprint + ":" + strconv.Itoa(durationSecs)
    if c.cache != nil {
        if v, ok := c.cache.Get(cacheKey); ok {
            var mbids []string
            _ = json.Unmarshal(v, &mbids)
            return mbids, nil
        }
    }

    // ... existing HTTP call and decode ...

    if c.cache != nil {
        if b, err := json.Marshal(mbids); err == nil {
            c.cache.Set(cacheKey, b, acoustidCacheTTL)
        }
    }
    return mbids, nil
}
```

`NewTestAcoustIDClient` keeps `cache: nil` (no change to its signature; tests that use `httptest.Server` don't need caching).

### `internal/adapters/identifier/music.go` — `NewMusicIdentifier`

Add `*cache.Cache` parameter and pass it to `newAcoustIDClient`:

```go
func NewMusicIdentifier(
    extIDs ports.ExternalIDRepository,
    items ports.ItemRepository,
    sources []ports.MetadataSource,
    acoustidKey string,
    acoustidCache *cache.Cache,
) ports.FileIdentifier {
    var ac *acoustidClient
    if acoustidKey != "" {
        ac = newAcoustIDClient(acoustidKey, acoustidCache)
    }
    ...
}
```

### `cmd/purser/main.go`

Create the `acoustid` cache and pass it through:

```go
acoustidCache, err := cache.New("acoustid", 512)
if err != nil { ... }
```

Pass `acoustidCache` as the last argument to `identifier.NewMusicIdentifier(...)` (line 122).

Add `acoustidCache` to the slice passed to `api.New(...)` (line 154) so it appears in the `/api/v1/cache/` stats endpoint.

---

## Phase 3 — Fix `ListUnmatchedGrouped` grouping

**Bug:** `ListUnmatchedGrouped` groups files by `item.GroupID` — the internal album ID of the library item the candidate points to. For files not yet in the library (the entire point of the unmatched queue), the top candidate has no `Item` (`best.Item == nil`), so every such file falls into the catch-all empty-group bucket. The queue renders all unidentified music as one unlabelled group instead of per-album buckets.

**Root cause:** `scan/service.go:839` — when `best.Item == nil`, the code falls through to `addToGroup("", "", uf, best.Confidence)` instead of using the candidate's `ExternalItem.GroupExternalID`.

**File to change:** `internal/app/scan/service.go`, function `ListUnmatchedGrouped` (~line 796).

Replace the inner loop body:

```go
// Before
for _, uf := range files {
    if len(uf.Candidates) == 0 {
        addToGroup("", "", uf, 0)
        continue
    }
    best := uf.Candidates[0]
    if best.Item == nil {
        addToGroup("", "", uf, best.Confidence)  // ← BUG: ignores ExternalItem.GroupExternalID
        continue
    }
    item, err := s.items.Get(ctx, best.Item.ID)
    if err != nil {
        addToGroup("", "", uf, best.Confidence)
        continue
    }
    groupTitle := ""
    if item.GroupID != "" && s.groups != nil {
        if grp, err := s.groups.Get(ctx, item.GroupID); err == nil {
            groupTitle = grp.Title
        }
    }
    addToGroup(item.GroupID, groupTitle, uf, best.Confidence)
}
```

```go
// After
for _, uf := range files {
    if len(uf.Candidates) == 0 {
        addToGroup("", "", uf, 0)
        continue
    }
    best := uf.Candidates[0]

    // Prefer external album ID (pre-import files). Fall back to the local group
    // when the candidate already links to an imported library item.
    if ext := best.ExternalItem; ext != nil && ext.GroupExternalID != "" {
        groupTitle := ext.GroupTitle  // populated by enrichFromTags / MBZ fetch
        addToGroup(ext.GroupExternalID, groupTitle, uf, best.Confidence)
        continue
    }
    if best.Item != nil {
        item, err := s.items.Get(ctx, best.Item.ID)
        if err != nil {
            addToGroup("", "", uf, best.Confidence)
            continue
        }
        groupTitle := ""
        if item.GroupID != "" && s.groups != nil {
            if grp, err := s.groups.Get(ctx, item.GroupID); err == nil {
                groupTitle = grp.Title
            }
        }
        addToGroup(item.GroupID, groupTitle, uf, best.Confidence)
        continue
    }
    addToGroup("", "", uf, best.Confidence)
}
```

**Domain check:** `ExternalItem.GroupExternalID` (string) exists at `internal/domain/metadata.go:18`. `ExternalItem.GroupTitle` may not exist — check `domain.ExternalItem` struct and add the field if absent, then populate it in `enrichFromTags` (music.go) and the MBZ recording fetch path.

**Tests to add** in `internal/app/scan/service_test.go`: one test where the top candidate has `ExternalItem.GroupExternalID = "release-mbid-1"` and `Item = nil`; assert the returned group's `GroupID` equals `"release-mbid-1"`. One test where `ExternalItem == nil` and `Item` is populated; assert grouping uses `item.GroupID`.

---

## Phase 4 — Error on missing album instead of creating orphan

**Bug:** In `importItemContainers` (service.go ~line 265), when `albumExtID` is empty and `ext.Studio == nil`, the function returns an `ImportItemResult` with no `Entry` and no `Album`. The caller (`ImportItem`) then saves an `Item` with `LibraryEntryID = ""` — a floating item with no parent. It will never appear in any entry's item list.

**Current code** (service.go ~line 292):

```go
albumExtID := req.AlbumExternalID
if albumExtID == "" {
    albumExtID = ext.GroupExternalID
}
if albumExtID != "" && result.Entry != nil {
    // import album
}
// if both are empty: silently returns result with no Album, no Entry
```

**Fix:** Return an error when both `ext.Studio` is nil (no parent entry to link to) and there is no album external ID to resolve the entry from. Music files always have both a studio (artist) and a group (album); if either is missing the external data is incomplete and we should not save a dangling item.

```go
if ext.Studio == nil {
    return nil, fmt.Errorf("import item: no parent entry (artist/studio) in external metadata for %q", ext.ExternalID)
}
```

Add this check immediately after the `ext.Studio` block in `importItemContainers`, before the album section. The error surfaces to the `ImportItem` caller and from there to the API handler as a 400 validation error (the `errs.IsValidation` check already exists in `importItem` handler).

**Wrap the error** with `errs.Validation(...)` so the handler returns 400 not 500:

```go
import "purser/internal/errs"
// ...
return nil, errs.Validation(fmt.Errorf("no parent entry in external metadata for %q — import the artist first or re-scan with complete tags", ext.ExternalID))
```

**Tests:** One test where `ext.Studio == nil`; assert `ImportItem` returns a non-nil error. One test where `ext.Studio` is populated but `albumExtID` is empty; assert the item is saved with a non-empty `LibraryEntryID` (entry created from ext.Studio) and `GroupID = ""` (no album — acceptable for scenes).

---

## Phase 5 — Async job ID from items/import + fix "View item" Link

This phase has two independent sub-tasks.

### 5a — Async job response from `POST /metadata/items/import`

**Bug:** `ImportItem` calls `FetchGroupContent` on MBZ, which is a network call to list all recordings in a release. On first call the response is not cached (cache is request-URL keyed in the MBZ adapter, not issue-specific), so it blocks until MBZ responds — which can take 2–5s. The UI hangs with no feedback.

**Design:** The handler should detect slow operations and return a job ID the UI polls, rather than blocking. The existing `POST /api/v1/commands` + `GET /api/v1/commands/{id}` infrastructure handles this.

**API change:**

`POST /api/v1/metadata/items/import` continues to behave synchronously for fast paths (item already exists, all data cached). When the operation takes longer than a threshold (or unconditionally for all import-from-external calls), return:

```json
HTTP 202 Accepted
{ "job_id": "uuid" }
```

The UI polls `GET /api/v1/commands/{job_id}` until `status == "done"` or `"error"`. On `"done"`, the command result carries `{ "item_id": "uuid" }`. The UI then calls `POST /api/v1/unmatched-files/{id}/match` with `{ "item_id": "..." }`.

**Implementation:**

In `importItem` handler (`internal/api/metadata.go`), submit a job via `h.svc.SubmitRefreshJob` pattern (or a new job type `ImportItem`). The job payload carries the same `ImportItemRequest` fields. The job worker calls `h.svc.ImportItem` and stores the resulting item ID in the job result.

If `h.jobs == nil` (test mode), fall back to synchronous execution and return the existing 201 response.

**UI change** (`web/src/components/scan/CreateFromCandidateDialog.tsx`):

In `handleCreate`, after `await importItem(...)`, check the response:

```ts
const resp = await importItem({ ... })
if ('job_id' in resp) {
  // poll GET /api/v1/commands/{resp.job_id} with exponential backoff
  // starting at 500ms, max 8 retries, cap at 10s
  itemId = await pollForItemId(resp.job_id)
} else {
  itemId = resp.item.id
}
```

Add a `pollForItemId` helper in `web/src/api/scan.ts` (or a new `web/src/api/commands.ts`).

### 5b — Fix "View item" plain anchor

**Bug:** `CreateFromCandidateDialog.tsx:146` uses `<a href={`/items/${createdId}`}>` — a plain anchor that causes a full page reload instead of a React Router client-side navigation.

**Fix:** Replace with `<Link to={`/items/${createdId}`}>` from `react-router-dom`.

```tsx
// Before
import { X, CheckCircle, Loader2, AlertCircle } from 'lucide-react'
// ...
<a href={`/items/${createdId}`} className="text-sm text-indigo-400 ...">View item</a>

// After
import { Link } from 'react-router-dom'
// ...
<Link to={`/items/${createdId}`} className="text-sm text-indigo-400 ...">View item</Link>
```

---

## Phase 6 — Computed composite confidence scoring

**Bug:** Confidence values are hardcoded per strategy:
- MBZ track ID tag: 0.99
- AcoustID: 0.95
- Tag fuzzy (≥2 matches): 0.92; single match: 0.75
- Filename: 0.40

A file with an AcoustID hit where title, artist, album, and duration all agree should score higher than one where only the fingerprint matched. A tag fuzzy match with four agreeing signals can be more reliable than a bare AcoustID hit.

**Target:** Score is computed as a base from the finding strategy plus signal-agreement bonuses, capped at 1.0.

| Strategy | Base |
|---|---|
| MBZ track ID tag (`musicbrainz_track_id`) | 0.95 |
| AcoustID → MBID | 0.80 |
| Tag fuzzy (multiple items) | 0.60 |
| Tag fuzzy (single item) | 0.70 |
| Filename parse | 0.35 |

| Signal bonus (each, additive) | Value |
|---|---|
| Candidate title matches embedded title tag (case-insensitive) | +0.05 |
| Candidate artist/studio matches embedded artist tag | +0.05 |
| Candidate album matches embedded album tag | +0.05 |
| Candidate duration within 5s of embedded duration | +0.05 |

Max possible: 0.95 + 0 bonuses (tag already had a specific MBID) = 0.95; AcoustID + all four bonuses = 1.00.

**File to change:** `internal/adapters/identifier/music.go`.

Extract a `computeConfidence(base float64, candidate domain.MatchCandidate, fp *domain.Fingerprint) float64` function. Call it from each strategy after producing a candidate. Move the per-strategy constants out of the inline literal and into named constants at the top of the file.

The `enrichFromTags` function already sets candidate title/artist/album from embedded tags when `ExternalItem` is present — the bonus computation can read from `candidate.ExternalItem` directly.

```go
const (
    baseMBZTrackID = 0.95
    baseAcoustID   = 0.80
    baseTagFuzzy1  = 0.70
    baseTagFuzzyN  = 0.60
    baseFilename   = 0.35
    bonusTitle     = 0.05
    bonusArtist    = 0.05
    bonusAlbum     = 0.05
    bonusDuration  = 0.05
)

func computeConfidence(base float64, c domain.MatchCandidate, fp *domain.Fingerprint) float64 {
    score := base
    if c.ExternalItem != nil {
        title := strings.ToLower(fp.EmbeddedTags["title"])
        if title != "" && strings.EqualFold(c.ExternalItem.Title, title) {
            score += bonusTitle
        }
        // ... artist, album, duration checks ...
    }
    if score > 1.0 {
        score = 1.0
    }
    return score
}
```

**Tests:** Unit tests for `computeConfidence` covering: base-only, all-bonuses, cap at 1.0, mismatched signals (no bonus). One integration test per strategy asserting the returned confidence is in expected range rather than exactly 0.95.

---

## Phase 7 — "Import Album" button in grouped queue view

**Bug:** "Accept All" in `AlbumUnmatchedGroup` is only enabled when `allHaveItems` — every file's top candidate must already link to an existing library item. When the album doesn't exist in the library yet, the entire group has no actionable button (other than per-file "Import & Create").

**Target:** An "Import Album" button in the group header that:
1. Calls `POST /api/v1/metadata/albums/import` with the release MBID from the group's best candidate
2. Polls until the album and all tracks are imported (Phase 5 job polling)
3. Then calls `POST /api/v1/unmatched-files/{id}/match` for each file in the group, matching each to the correct track by position/title

**Prerequisite:** Phase 3 must be done (groups must carry the album MBID as `group_id`). Phase 5 job polling must be available.

**Server:** No new endpoints needed. The sequence is:
- `POST /api/v1/metadata/albums/import` — returns `job_id` (Phase 5 pattern)
- Poll `GET /api/v1/commands/{job_id}` for completion
- Result carries the album's `Group` record including all imported tracks
- For each file in the unmatched group, find the matching track by title or sequence, call `POST /api/v1/unmatched-files/{id}/match`

**UI change:** `web/src/components/scan/AlbumUnmatchedGroup.tsx`

Add `group_external_id` to `UnmatchedFileGroup` TypeScript interface (it needs to come through the API; check `domain.UnmatchedFileGroup` in `internal/domain/scan.go` and the `toUnmatchedGroupResponse` serialiser in the API).

Add state: `importing: boolean`, `importError: string | null`.

Show "Import Album" button when:
- `group.group_id` is non-empty (we have the MBID)
- NOT `allHaveItems` (album doesn't exist yet)

Button handler:
```ts
const handleImportAlbum = async () => {
  setImporting(true)
  try {
    const resp = await importAlbum({ source: 'musicbrainz', externalId: group.group_id, ... })
    const albumResp = 'job_id' in resp ? await pollForResult(resp.job_id) : resp
    // match each file to its track
    await Promise.all(
      group.files.map(f => matchFileToTrack(f, albumResp.tracks))
    )
    onResolved()
  } catch (e) {
    setImportError((e as Error).message)
  } finally {
    setImporting(false)
  }
}
```

`matchFileToTrack` compares the file's best candidate `ExternalItem.Title` / sequence against the album's track list.

---

## Phase 8 — Candidate selection UI

**Bug:** In `UnmatchedFileCard` (and `UnmatchedFileDetail`), only the top candidate is actionable. When the top candidate is wrong, the user has no way to pick a lower-ranked candidate without re-scraping.

**Target:** Each candidate row in the detail view is selectable. The selected candidate is used for "Accept" and "Import & Create" instead of always using `candidates[0]`.

**UI change:** `web/src/components/scan/UnmatchedFileDetail.tsx`

Add state: `selectedIdx: number` (defaults to 0).

Candidate rows get a radio-button or highlight style when selected. Clicking a row sets `selectedIdx`.

"Accept" and "Import & Create" buttons use `candidates[selectedIdx]` instead of `candidates[0]`.

No API change needed — `manualMatch` and `importItem` already accept arbitrary item IDs / external IDs; the selection is purely client-side.

---

## Phase 9 — Album thumbnail from Cover Art Archive

**Bug:** Album groups in the queue header show no cover art. The `UnmatchedFileGroup` type carries no image field. Cover Art Archive serves release artwork by MBID at `https://coverartarchive.org/release/{mbid}/front-250`.

**Target:** Each album group header shows a 40×40 cover thumbnail fetched by MBID.

**Server:** No new endpoint. The image is fetched client-side since Cover Art Archive is a public CDN with permissive CORS and no API key requirement.

**TypeScript type:** Add `cover_url?: string` to `UnmatchedFileGroup` in `web/src/types/index.ts`. Populate it server-side in the unmatched-groups API response from the Cover Art Archive URL template when `group_id` (the release MBID) is non-empty:

```
cover_url = "https://coverartarchive.org/release/" + groupID + "/front-250"
```

Set it in the Go `toUnmatchedGroupResponse` serialiser — no network call needed, just string construction. The client handles 404 gracefully (image `onError` falls back to a placeholder).

**UI change:** `web/src/components/scan/AlbumUnmatchedGroup.tsx`

In the group header, add a 40×40 `<img>` before the title. Use `onError` to hide it when the CAA returns 404 (rare releases without artwork):

```tsx
{group.cover_url && (
  <img
    src={group.cover_url}
    alt=""
    className="w-10 h-10 rounded object-cover shrink-0 bg-white/5"
    onError={e => { (e.target as HTMLImageElement).style.display = 'none' }}
  />
)}
```

---

## Phase 10 — End-to-end tests

**Goal:** A suite of tests that exercises the full scan → fingerprint → identify → queue → import path without real network calls and without real audio files.

**Design decisions:**
- One shared `testHarness` struct that wires all real in-memory components (scan.Service + metadata.Service + all repos), swapping only external HTTP clients for fixtures
- Fixture responses stored as JSON files in `internal/app/testdata/` (one file per external response)
- Fictional entities only: "The Boozy Fig Digglers" (music), "Naughty Salamander Productions" (adult), "Cardboard Box Mysteries" (TV), "Attack of the Phantom Ledger" (movie)
- No real audio files: fingerprint is injected directly via `domain.Fingerprint` (the test skips the `FileFingerprinter` chain and calls `scan.Service` with a pre-built `ScannedFile.Fingerprint`)

**Harness location:** `internal/app/testdata/` for JSON fixtures; harness code in `internal/app/scan/e2e_test.go` (or a `_test` package if cross-service).

**Test harness structure:**

```go
type testHarness struct {
    scanSvc  *scan.Service
    metaSvc  *metadata.Service
    items    *stubItemRepo       // in-memory
    entries  *stubEntryRepo      // in-memory
    groups   *stubGroupRepo      // in-memory
    unmatched *stubUnmatchedRepo // in-memory
    extIDs   *stubExternalIDRepo
    mbz      *httptest.Server    // serves JSON fixtures
    acoustid *httptest.Server    // serves JSON fixtures
}
```

**Fixture files** (examples):

`testdata/mbz_recording_boozy_fig_001.json` — MBZ recording response for a Boozy Fig Digglers track
`testdata/acoustid_lookup_boozy_fig_001.json` — AcoustID response returning the above MBID
`testdata/mbz_release_boozy_fig_album1.json` — MBZ release response for their debut album

**Test cases:**

1. **Music above threshold auto-imports:**
   - ScannedFile with fingerprint `{ AcoustID: "fp-001", EmbeddedTags: { musicbrainz_track_id: "rec-001" } }`
   - MBZ fixture returns Boozy Fig Digglers recording with GroupExternalID = "release-001"
   - Confidence ≥ 0.85 → expect `MediaFile` created, item status = imported, unmatched queue empty

2. **Music below threshold goes to queue, grouped correctly:**
   - Two ScannedFiles with AcoustID hits but no embedded MBID tags → confidence ~0.80
   - Both candidates carry `GroupExternalID = "release-001"`
   - Assert queue has one group with `GroupID = "release-001"` containing both files

3. **Import & Create flow:**
   - Start from an unmatched file in queue
   - Call `scan.Service.Match` (manual match) after calling `metadata.Service.ImportItem`
   - Assert item created with correct `LibraryEntryID` (artist entry), `GroupID` (album)
   - Assert unmatched file status = matched

4. **No parent entry returns error:**
   - ScannedFile whose MBZ fixture returns a recording with `Studio = nil`
   - Assert `ImportItem` returns a validation error (Phase 4)

5. **Idempotent re-scan:**
   - Run the same ScannedFile through the pipeline twice
   - Assert only one `MediaFile` record exists after the second scan

**Run gate:** Tests must pass with `go test ./internal/app/...` with no network access (use `httptest.Server` for all external calls). Tag with `//go:build !integration` so they run in CI without special flags.
