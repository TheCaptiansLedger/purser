# Music Implementation Plan

> Tasks are ordered by dependency and grouped into phases. Each task maps to one GitHub issue.
> Every task must compile and pass `go test ./...` / `npx tsc --noEmit` before the PR merges.
>
> ## Design verification principle
>
> The API CRUD layer comes in Task 6 — immediately after storage. From that point on, every
> backend task has a "Verified by curl" section: copy-pasteable curl commands that prove the
> task's output is correct without touching the UI. Tasks 1–5 are plumbing; Task 6 is the
> first moment you can stand on the pipeline and confirm the data model is what you designed.
> If Task 6 curl commands fail, stop — fix the data model before Task 7.
>
> Logging is required on every task that "does a thing." Log lines listed in each task are
> mandatory — they are how you debug a broken pipeline before the UI exists.
>
> Reference docs:
> - `docs/technical/music_organization.md` — pipeline and identification flow
> - `docs/technical/music_data_model.md` — entity definitions, ports, storage notes

---

## Phase 1 — Domain, Ports, and Storage Foundation

### Task 1: Domain types

**Goal**: All new and modified domain types. Everything depends on this.

**Files**:
- `internal/domain/music.go` (new) — `MusicRelease`, `ReleaseStatus` (`stub`/`partial`/`imported`)
- `internal/domain/scan.go` (update) — add `MusicScanGroup`, `MusicTagSummary`, `MusicReleaseCandidate`, `MusicConfidenceSignals`; update `MusicMatchDetail` to embed `MusicConfidenceSignals`
- `internal/domain/media_file.go` (update) — add `SHA1 string`; add `Metadata map[string]string`
- `internal/domain/external_id.go` (update) — add `SourceMBZRecording ExternalIDSource = "mbz_recording"`

**Tests** (`internal/domain/`):
- `TestMusicRelease_ZeroValueSurvivesApplyDefaults`
- `TestReleaseStatus_DistinctConstants`
- `TestMusicConfidenceSignals_AllFieldsZero`
- `TestMusicScanGroup_ReusesUnmatchedStatus`
- `TestMediaFile_SHA1RoundTrip`
- `TestMediaFile_MetadataRoundTrip`

**Verified by**: `go build ./...` and `go vet ./internal/domain/...` both pass clean.

---

### Task 2: Ports

**Goal**: New port interfaces. No implementations.

**Files**:
- `internal/ports/music.go` (new) — `MusicReleaseRepository`, `MusicScanGroupRepository`
- `internal/ports/metadata_source.go` (update) — add `ReleaseGroupContentSource`, `ISRCLookupSource`, `ExternalMusicRelease` data-transfer type
- `internal/ports/scan.go` (update) — add `FileGrouper` (`Group(ctx, []ScannedFile) ([]ScannedFileGroup, error)`) and `GroupIdentifier` (`Identify(ctx, ScannedFileGroup) error`) interfaces

`MusicReleaseRepository` methods: `Get`, `GetByMBID`, `GetByBarcode`, `ListByGroup`, `ListByEntry`, `ListTracksByRelease`, `Save`, `Delete`.

`MusicScanGroupRepository` methods: `Get`, `List(status)`, `Save`, `Delete`.

`ReleaseGroupContentSource` (optional, type-asserted):
```go
type ReleaseGroupContentSource interface {
    FetchReleaseGroupReleases(ctx context.Context, rgMBID string) ([]*ExternalMusicRelease, error)
}
```

`ExternalMusicRelease` carries: MBID, title, country, date, label, catalog number, barcode, format, medium count, track count, is-default flag.

**Tests**: Compile-time interface checks (`var _ ports.X = (*stub)(nil)`). `TestReleaseGroupContentSource_IsOptional` — struct that only implements `MetadataSource` must fail the type assertion.

**Verified by**: `grep -r "badger\.\|db\." internal/app/` returns zero hits.

---

### Task 3: Contract tests

**Goal**: Shared contract suite extended for new repos and `MediaFile` changes. Both adapters must pass after Tasks 4 and 5.

**Files**:
- `internal/adapters/contract/music_release.go` (new)
- `internal/adapters/contract/music_scan_group.go` (new)
- `internal/adapters/contract/media_file.go` (update) — add `SHA1RoundTrip`, `MetadataRoundTrip`, `NilMetadataRoundTrip`
- `internal/adapters/contract/contract.go` (update) — add `MusicReleases` and `MusicScanGroups` to `BackendSuite`; call new runners in `RunAll`

`runMusicReleaseContract` sub-tests: `SaveAndGet`, `GetByMBID`, `GetByBarcode`, `GetByMBIDNotFound` → `ErrNotFound`, `GetByBarcodeNotFound` → `ErrNotFound`, `ListByGroup`, `ListByEntry`, `ListTracksByRelease`, `StatusRoundTrip`, `IsDefaultRoundTrip`, `Delete`, `StaleIndexCleanedOnResave` (update barcode A → B; `GetByBarcode("A")` must return `ErrNotFound`).

`runMusicScanGroupContract` sub-tests: `SaveAndGet`, `ListByStatus`, `UpdateStatus`, `Delete`, `CandidatesRoundTrip` — all seven `MusicConfidenceSignals` float64 fields must survive storage round-trip without precision loss or missing keys.

**Verified by**: `go test ./internal/adapters/contract/... -list .` shows all new test names. Running against a stub adapter that returns `ErrNotFound` everywhere fails loudly on every sub-test.

---

### Task 4: Storage — MusicRelease (BadgerDB + SQLite) and MediaFile updates

**Goal**: Implement `MusicReleaseRepository` in both adapters; persist `SHA1` and `Metadata` on `MediaFile`.

**Files**:
- `internal/adapters/badger/music_release.go` (new)
- `internal/adapters/db/music_release.go` (new)
- `internal/adapters/db/migrations/NNNN_music_releases.sql` (new)
- `internal/adapters/badger/media_file.go` (update)
- `internal/adapters/db/media_file.go` (update)
- `internal/adapters/db/migrations/NNNN_media_file_sha1_metadata.sql` (new)

BadgerDB key layout: `mrel:{id}`; secondary indexes: `mrel:grp:{group_id}:{id}`, `mrel:entry:{entry_id}:{id}`, `mrel:mbid:{mbid}`, `mrel:barcode:{barcode}`. On re-save, read old record first and delete stale index keys atomically. `ListTracksByRelease` scans item prefix and filters by `Metadata["release_id"]` — documented trade-off until a dedicated index is added.

**Verified by**:
- `go test ./internal/adapters/badger/... -run TestBadger_Contract/MusicRelease` — all sub-tests pass
- `go test ./internal/adapters/db/... -run TestDB_Contract/MusicRelease` — all sub-tests pass
- `go test ./internal/adapters/badger/... -run TestBadger_Contract/MediaFile` — `SHA1RoundTrip` and `MetadataRoundTrip` pass

---

### Task 5: Storage — MusicScanGroup (BadgerDB + SQLite)

**Goal**: Implement `MusicScanGroupRepository` in both adapters.

**Files**:
- `internal/adapters/badger/music_scan_group.go` (new)
- `internal/adapters/db/music_scan_group.go` (new)
- `internal/adapters/db/migrations/NNNN_music_scan_groups.sql` (new)

BadgerDB: record key `msg:{id}`; status index `msg:status:{status}:{id}`. SQLite: `music_scan_groups` table with `files`, `tags`, `candidates` as JSON blobs.

**Verified by**:
- `go test ./internal/adapters/badger/... -run TestBadger_Contract/MusicScanGroup` — all sub-tests pass including `CandidatesRoundTrip`
- `go test ./internal/adapters/db/... -run TestDB_Contract/MusicScanGroup` — all sub-tests pass

---

## Phase 2 — Minimal API (Design Verification Gate)

### Task 6: Minimal API — MusicRelease and MusicScanGroup CRUD

**Goal**: Create just enough API surface to verify the data model and storage with curl before any pipeline work starts. This is the design verification gate. Every subsequent backend task has a "Verified by curl" section that uses these endpoints. If the curl commands here fail, stop — fix the data model before Task 7.

**Files**:
- `internal/api/music_releases.go` (new)
- `internal/api/music_queue.go` (new)
- `internal/api/server.go` (update) — register routes
- `internal/api/types.go` (update) — `MusicReleaseResponse`, `MusicScanGroupResponse`, `MusicConfidenceSignalsResponse`, `MusicReleaseCandidateResponse`

**Routes** (more are added in Task 18):
```
POST   /api/music/releases
GET    /api/music/releases/{id}
GET    /api/groups/{id}/releases
PATCH  /api/music/releases/{id}
DELETE /api/music/releases/{id}

POST   /api/music/queue
GET    /api/music/queue               ?status=pending|matched|dismissed
GET    /api/music/queue/{id}
DELETE /api/music/queue/{id}
```

`MusicReleaseResponse` must include all fields: `id`, `groupId`, `libraryEntryId`, `title`, `country`, `date`, `label`, `catalogNumber`, `barcode`, `format`, `mediumCount`, `trackCount`, `isDefault`, `monitored`, `status`, `externalIds`, `coverUrl`, `addedAt`, `updatedAt`.

`MusicConfidenceSignalsResponse` must expose all seven signal fields as JSON numbers: `barcode`, `isrc`, `rgNameFuzzy`, `trackCount`, `trackTitleSet`, `duration`, `acoustid`.

**Tests** (`internal/api/server_test.go`):
- `TestAPI_MusicRelease_CreateAndGet`
- `TestAPI_MusicRelease_NotFound_Returns404`
- `TestAPI_MusicRelease_PatchStatus`
- `TestAPI_MusicRelease_PatchMonitored`
- `TestAPI_MusicRelease_Delete`
- `TestAPI_MusicQueue_CreateAndGet`
- `TestAPI_MusicQueue_ListByStatus`
- `TestAPI_MusicQueue_Detail_AllSevenSignalFieldsPresent` — POST a queue entry with all seven signals set; GET detail; assert all seven appear as number fields (not null, not missing)
- `TestAPI_MusicQueue_Dismiss`

**Verified by curl** (run against a live server at `http://localhost:7474` after `go run ./cmd/purser`):

```bash
# Step 1: Create a test artist and release group (reusing existing endpoints)
ENTRY_ID=$(curl -s -X POST http://localhost:7474/api/library \
  -H "Content-Type: application/json" \
  -d '{"kind":"artist","contentType":"music","name":"Test Artist"}' | jq -r '.id')

GROUP_ID=$(curl -s -X POST http://localhost:7474/api/groups \
  -H "Content-Type: application/json" \
  -d "{\"libraryEntryId\":\"$ENTRY_ID\",\"title\":\"Test Album\",\"year\":1980}" | jq -r '.id')

# Step 2: Create a MusicRelease — verify all fields round-trip
RELEASE_ID=$(curl -s -X POST http://localhost:7474/api/music/releases \
  -H "Content-Type: application/json" \
  -d "{\"groupId\":\"$GROUP_ID\",\"libraryEntryId\":\"$ENTRY_ID\",\"title\":\"Test Album (Original)\",\"country\":\"US\",\"date\":\"1980-01-01\",\"barcode\":\"012345678901\",\"format\":\"CD\",\"mediumCount\":1,\"trackCount\":10,\"isDefault\":true,\"status\":\"stub\"}" \
  | jq -r '.id')

curl -s http://localhost:7474/api/music/releases/$RELEASE_ID \
  | jq '{id, title, country, date, barcode, format, mediumCount, trackCount, isDefault, status}'
# MUST return all fields. Any null or missing field = data model bug.

# Step 3: List by group
curl -s "http://localhost:7474/api/groups/$GROUP_ID/releases" | jq 'length'
# → 1

# Step 4: PATCH status
curl -s -X PATCH http://localhost:7474/api/music/releases/$RELEASE_ID \
  -H "Content-Type: application/json" \
  -d '{"status":"imported"}' | jq '.status'
# → "imported"

# Step 5: PATCH monitored
curl -s -X PATCH http://localhost:7474/api/music/releases/$RELEASE_ID \
  -H "Content-Type: application/json" \
  -d '{"monitored":true}' | jq '.monitored'
# → true

# Step 6: POST a queue entry with all seven signals
QUEUE_ID=$(curl -s -X POST http://localhost:7474/api/music/queue \
  -H "Content-Type: application/json" \
  -d '{"folderPath":"/music/test","totalTracks":10,"totalDiscs":1,"status":"pending","candidates":[{"artistName":"Test Artist","releaseGroupTitle":"Test Album","releaseTitle":"Test Album (Original)","overallConfidence":0.72,"signals":{"barcode":0.0,"isrc":0.95,"rgNameFuzzy":0.62,"trackCount":0.20,"trackTitleSet":0.45,"duration":0.88,"acoustid":0.0}}]}' \
  | jq -r '.id')

# Step 7: Verify ALL SEVEN signal fields are present — this is the critical check
curl -s http://localhost:7474/api/music/queue/$QUEUE_ID \
  | jq '.candidates[0].signals | keys_unsorted'
# → ["acoustid","barcode","duration","isrc","rgNameFuzzy","trackCount","trackTitleSet"]
# All seven must be present. Missing any = serialization bug.

curl -s http://localhost:7474/api/music/queue/$QUEUE_ID \
  | jq '.candidates[0].signals | to_entries | map(select(.value == null)) | length'
# → 0 (no null values)

# Step 8: Confirm list filter works
curl -s "http://localhost:7474/api/music/queue?status=pending" | jq 'length'
# → 1

curl -s "http://localhost:7474/api/music/queue?status=matched" | jq 'length'
# → 0

# Step 9: Delete
curl -s -X DELETE http://localhost:7474/api/music/releases/$RELEASE_ID
curl -s http://localhost:7474/api/music/releases/$RELEASE_ID | jq '.error // .message'
# → contains "not found"
```

**If steps 7 fails (missing signal key or null value)**: the `MusicConfidenceSignals` struct is not being serialized correctly. Fix before proceeding — every downstream task depends on this shape being correct.

---

## Phase 3 — Fingerprinting and Tag Extraction

### Task 7: Music fingerprinter — complete tag extraction

**Goal**: Extract barcode, ISRC, label, track/disc totals, and all MBZ IDs from FLAC and ID3v2 tags.

**Files**:
- `internal/adapters/fingerprint/music.go` (update) — extend `mbzKeyMap` and `readTags`
- `internal/adapters/fingerprint/music_test.go` (update)
- `internal/adapters/fingerprint/music_integration_test.go` (update)

New `EmbeddedTags` keys: `barcode`, `isrc`, `label`, `catalog_number`, `track_total`, `disc_total`, `musicbrainz_album_id`, `musicbrainz_release_group_id`, `musicbrainz_album_artist_id`.

**Logging requirements**:
- `DEBUG` after each file fingerprinted: `"music fingerprint" path=<path> barcode=<value|not_set> isrc=<value|not_set> mbz_album_id=<value|not_set> tag_count=<N>`
- `WARN` when `fpcalc` not found: `"fpcalc not found, acoustid disabled"`

**Tests**:
- `TestMusicFingerprinter_TagTable` — table-driven; one row per new key
- `TestMusicFingerprinter_Integration_REOSpeedwagon_Track1` — real FLAC; `barcode == "0074646161425"`, `isrc == "USSM10012807"`, `label == "Epic - Legacy"`, `track_total == "10"`, `disc_total == "1"`
- `TestMusicFingerprinter_Integration_AllTracks_HaveISRC` — all 10 FLACs have non-empty `isrc`

**Verified by**:
- `go test -run TestMusicFingerprinter_Integration_REOSpeedwagon_Track1 ./internal/adapters/fingerprint/... -v` — output shows full extracted tag map via `t.Logf`
- Start server, scan `test-data/music/` with `LOG_LEVEL=debug`: `grep "music fingerprint" server.log | head -3` shows `barcode=0074646161425 isrc=USSM10012807` on track 1

---

## Phase 4 — Scanning Model: Folder Grouping

### Task 8: FileGrouper — music folder grouper

**Goal**: Group music files by folder into `ScannedFileGroup` slices. Detect multi-disc layouts (CD1/CD2, Disc N) and merge sub-folders under a common root. Wire `FileGrouper` into the scan service without any content-type branch.

**Files**:
- `internal/adapters/identifier/music_grouper.go` (new)
- `internal/adapters/identifier/music_grouper_test.go` (new)
- `internal/app/scan/service.go` (update) — accept `[]ports.FileGrouper`; fan-out after fingerprinting; pass groups to `GroupIdentifier`
- `internal/app/scan/service_test.go` (update)

Multi-disc patterns: `CD\d+`, `Disc \d+`, `Disk \d+` (case-insensitive) as direct siblings of a common parent. Merge all into one group; `RootPath` = common parent.

**Logging requirements**:
- `INFO`: `"music folder grouper" total_files=<N> groups=<M>` — after each `Group` call
- `DEBUG` per group: `"music group formed" root=<path> tracks=<N> discs=<D> multi_disc=<bool>`
- `DEBUG` on multi-disc detection: `"multi-disc detected" root=<path> subdirs=[CD1,CD2]`

**Tests**:
- `TestMusicFolderGrouper_SingleDisc`
- `TestMusicFolderGrouper_MultiDisc_CD` — `Album/CD1/` and `Album/CD2/` → 1 group, `RootPath == "Album/"`
- `TestMusicFolderGrouper_MultiDisc_DiscN`
- `TestMusicFolderGrouper_TwoAlbums_TwoGroups`
- `TestMusicFolderGrouper_OnlyClaimsMusic`
- `TestScanService_RoutesGroupsThroughGrouper`

**Verified by**:
- `go test -run TestMusicFolderGrouper_MultiDisc_CD ./internal/adapters/identifier/... -v` — output shows merged group
- Scan `test-data/music/` with `LOG_LEVEL=debug`: `grep "music folder grouper"` shows `groups=1`; `grep "music group formed"` shows `tracks=10 discs=1`

---

### Task 9: MusicTagSummary extraction

**Goal**: Given a `ScannedFileGroup`, produce a `MusicTagSummary` from consensus tag values across all files in the group.

**Files**:
- `internal/adapters/identifier/music_tags.go` (new)
- `internal/adapters/identifier/music_tags_test.go` (new)

Consensus = majority value; ties broken by first non-empty. `TotalDiscs` falls back to count of distinct disc numbers when `disc_total` tag absent.

**Logging requirements**:
- `INFO`: `"music tag summary" root=<path> album_artist=<value> album=<value> year=<value> barcode=<value|""> total_tracks=<N> total_discs=<N> mbz_release_id=<value|""> has_isrcs=<bool>`
- `WARN` when no consensus exists for a key: `"tag consensus failed" root=<path> key=<key> values=[v1,v2,v3]`

**Tests**:
- `TestExtractMusicTagSummary_Consensus`
- `TestExtractMusicTagSummary_OutlierIgnored`
- `TestExtractMusicTagSummary_TracksOrderedByNumber`
- `TestExtractMusicTagSummary_MultiDisc_DerivesTotalDiscs`
- `TestExtractMusicTagSummary_MBZReleaseIDPresent`
- `TestExtractMusicTagSummary_NoConsensusEmitsWarn`
- Integration: `TestExtractMusicTagSummary_Integration_REOSpeedwagon` — `Barcode == "0074646161425"`, `TotalTracks == 10`, all 10 ISRCs non-empty

**Verified by**:
- `go test -run TestExtractMusicTagSummary_Integration_REOSpeedwagon ./internal/adapters/identifier/... -v` — prints full `MusicTagSummary` via `t.Logf`
- Scan `test-data/music/` with `LOG_LEVEL=info`: `grep "music tag summary"` shows `barcode=0074646161425 total_tracks=10 has_isrcs=true`

---

## Phase 5 — MBZ Adapter Expansion

### Task 10: MBZ — artist full enrichment

**Goal**: Fetch and populate all artist metadata keys: `artist_type`, `aliases`, `founded_date`, `founded_location`, `dissolved_date`, `isni`, `official_url`, `lastfm_url`, `wikipedia_url`. Solo-artist Person metadata: `born_date`, `born_location`, `died_date`.

**Files**:
- `internal/adapters/mbz/artist.go` (update)
- `internal/adapters/mbz/artist_test.go` (update)
- `internal/adapters/mbz/adapter_integration_test.go` (update)

MBZ request: `/ws/2/artist/{mbid}?inc=artist-rels+url-rels+aliases+isnis&fmt=json`

Alias filtering: type `"Artist name"` or `"Search hint"`, locale empty or `"en"`.

**Logging requirements**:
- `INFO`: `"mbz artist enriched" mbid=<mbid> name=<name> type=<type> aliases=<N> members=<N> url_relations=<N>`
- `DEBUG` per URL relation: `"mbz url relation" type=<type> url=<url>`
- `WARN` on unrecognised artist type: `"unknown artist type" mbid=<mbid> type=<value>`

**Tests**:
- `TestMBZArtist_ParsesAliases_FiltersTypes`
- `TestMBZArtist_ParsesLifeSpan_Group`
- `TestMBZArtist_ParsesLifeSpan_Person`
- `TestMBZArtist_ParsesURLRelations`
- `TestMBZArtist_ParsesISNI`
- Integration: `TestMBZAdapter_Integration_REOSpeedwagon_Enrichment`

**Verified by**:
- `go test -tags integration -run TestMBZAdapter_Integration_REOSpeedwagon_Enrichment ./internal/adapters/mbz/... -v` — prints populated metadata map
- After importing REO Speedwagon: `grep "mbz artist enriched" server.log` shows `aliases=<N>` non-zero and `members=<N>` ≥5

---

### Task 11: MBZ — all releases for a Release Group, barcode lookup, ISRC lookup

**Goal**: Implement `ReleaseGroupContentSource.FetchReleaseGroupReleases`, `GetReleaseByBarcode`, and `ISRCLookupSource.LookupISRC`.

**Files**:
- `internal/adapters/mbz/release.go` (update)
- `internal/adapters/mbz/isrc.go` (new)
- `internal/adapters/mbz/release_test.go` (update)
- `internal/adapters/mbz/adapter_integration_test.go` (update)

MBZ endpoints:
- All releases for RG: `/ws/2/release?release-group={rgMBID}&inc=labels+mediums&limit=100&fmt=json`
- Barcode: `/ws/2/release?query=barcode:{barcode}&fmt=json`
- ISRC: `/ws/2/isrc/{isrc}?inc=releases&fmt=json`

`IsDefault` heuristic: earliest-dated `Official` release; ties broken by MBZ result order.

**Logging requirements**:
- `INFO`: `"mbz rg releases fetched" rg_mbid=<mbid> count=<N> default_mbid=<mbid>`
- `DEBUG` per release: `"mbz release" mbid=<mbid> title=<title> country=<country> date=<date> barcode=<value|""> is_default=<bool>`
- `INFO`: `"mbz barcode lookup" barcode=<value> hit=<bool> release_mbid=<value|"">`
- `INFO`: `"mbz isrc lookup" isrc=<value> rg_mbid=<value|""> release_count=<N>`

**Tests**:
- `TestMBZRelease_ParsesAllEditions`
- `TestMBZRelease_IsDefaultEarliestOfficial`
- `TestMBZRelease_BarcodeReturnsRelease`
- `TestMBZISRC_ReturnsReleaseGroupMBID`
- Integration: `TestMBZAdapter_Integration_HiInfidelity_AllReleases` — ≥5 releases; 2024 Digital has barcode `074646161425`
- Integration: `TestMBZAdapter_Integration_BarcodeToRelease` — barcode `074646161425` → `1e639bf3-6b4c-4e1a-9d15-c61511804c8f`

**Verified by**:
- `go test -tags integration -run TestMBZAdapter_Integration_BarcodeToRelease ./internal/adapters/mbz/... -v` — prints resolved release MBID
- `grep "mbz barcode lookup" server.log` shows `hit=true` after a barcode scan
- `grep "mbz rg releases fetched" server.log` shows `count=<N>` ≥5 after fetching Hi Infidelity releases

---

## Phase 6 — Confidence Cascade

### Task 12: Confidence cascade — Release Group identification signals

**Goal**: Standalone testable signal functions for RG identification: barcode, ISRC, suffix-stripped fuzzy name, track count elimination.

**Files**:
- `internal/adapters/identifier/music_rg_signals.go` (new)
- `internal/adapters/identifier/music_rg_signals_test.go` (new)

Functions: `StripAlbumSuffixes(string) string`, `ScoreBarcodeSignal`, `ScoreISRCSignal`, `ScoreFuzzyNameSignal`, `FilterByTrackCount`.

**Logging requirements**:
- `INFO` per signal evaluated: `"rg signal" root=<path> signal=<name> score=<0.0-1.0>`
- `DEBUG` on suffix strip when title changes: `"suffix stripped" original=<value> stripped=<value>`
- `DEBUG` on fuzzy match: `"fuzzy candidates" root=<path> top=[{title,score},{title,score},{title,score}]`

**Tests**:
- `TestStripAlbumSuffixes` — 12-row table covering all patterns
- `TestScoreBarcodeSignal_Hit`, `TestScoreBarcodeSignal_Miss`
- `TestScoreISRCSignal_AllAgree`, `TestScoreISRCSignal_PartialAgreement`, `TestScoreISRCSignal_NoneAgree`
- `TestScoreFuzzyNameSignal_ExactAfterStrip`, `TestScoreFuzzyNameSignal_LiveVsStudio`
- `TestFilterByTrackCount_EliminatesNonMatching`

**Verified by**:
- `go test -run TestStripAlbumSuffixes ./internal/adapters/identifier/... -v` — all 12 rows pass
- Scan `test-data/music/` with `LOG_LEVEL=debug`: `grep "rg signal"` shows all four signals logged with their scores; `grep "suffix stripped"` shows `original="Hi Infidelity (2024 Remaster)" stripped="Hi Infidelity"`

---

### Task 13: Confidence cascade — Release identification signals

**Goal**: Standalone signal functions for release identification: track count elimination, year, per-track duration, per-track AcoustID.

**Files**:
- `internal/adapters/identifier/music_release_signals.go` (new)
- `internal/adapters/identifier/music_release_signals_test.go` (new)

Functions: `FilterReleasesByTrackCount`, `ScoreReleaseYearSignal`, `ScoreReleaseDurationSignal`, `ScoreReleaseAcoustIDSignal`, `RankReleases`.

**Logging requirements**:
- `INFO` per signal: `"release signal" release_mbid=<mbid> signal=<name> score=<0.0-1.0> matched=<N> total=<N>`
- `DEBUG` per track duration comparison: `"track duration" track=<N> file_ms=<N> mbz_ms=<N> ok=<bool>`
- `INFO` final ranking: `"release ranked" count=<N> top_mbid=<mbid> top_score=<0.0-1.0>`

**Tests**:
- `TestFilterReleasesByTrackCount_RemovesNonMatching`
- `TestScoreReleaseYearSignal_ExactMatch`, `TestScoreReleaseYearSignal_YearUnknown`, `TestScoreReleaseYearSignal_OffByOne`
- `TestScoreReleaseDurationSignal_AllMatch`, `TestScoreReleaseDurationSignal_OneOff`
- Integration: `TestScoreReleaseDurationSignal_Integration_REOSpeedwagon` — real FLACs vs MBZ track list; score ≥0.95
- `TestRankReleases_DefaultWinsTie`

**Verified by**:
- `go test -run TestScoreReleaseDurationSignal_Integration_REOSpeedwagon ./internal/adapters/identifier/... -v` — prints score and which tracks matched
- Scan `test-data/music/`: `grep "release signal" server.log` shows all signal lines; `grep "release ranked"` shows `top_score=<N>` and the winning MBID

---

### Task 14: Album-level identifier — full pipeline assembly

**Goal**: Assemble the complete album identifier using Tasks 9–13. On confidence ≥ threshold, call the release importer. On confidence < threshold, persist `MusicScanGroup` to the queue. Re-scan shortcut: if `MBZReleaseID` is in tags and the release is `imported`, return immediately.

**Files**:
- `internal/adapters/identifier/music_album.go` (new)
- `internal/adapters/identifier/music_album_test.go` (new)
- `internal/app/scan/service.go` (update) — wire `GroupIdentifier`

**Logging requirements**:
- `INFO`: `"album scan start" root=<path> tracks=<N> has_barcode=<bool> has_isrcs=<bool> has_mbz_id=<bool>`
- `INFO`: `"album re-scan shortcut" root=<path> release_id=<id>` — when shortcut fires
- `INFO`: `"album confidence" root=<path> overall=<0.0-1.0> threshold=<0.0-1.0> action=auto_import|queue`
- `INFO`: `"album queued" root=<path> queue_id=<id> top_candidate=<title> top_score=<0.0-1.0>`
- `INFO`: `"album auto-importing" root=<path> release_mbid=<mbid> confidence=<0.0-1.0>`

**Tests**:
- `TestMusicAlbumIdentifier_ReScanShortcut_SkipsKnownImport`
- `TestMusicAlbumIdentifier_ReScanShortcut_ProceedsIfNotImported`
- `TestMusicAlbumIdentifier_BarcodeHit_AutoImports`
- `TestMusicAlbumIdentifier_BelowThreshold_SavesQueue`
- `TestMusicAlbumIdentifier_CandidatesRankedByConfidence`
- `TestMusicAlbumIdentifier_AllSignalBreakdownPresent`

**Verified by curl** (after scanning `test-data/music/`):
```bash
# Trigger scan
curl -s -X POST http://localhost:7474/api/commands/scan \
  -H "Content-Type: application/json" \
  -d '{"paths":["/path/to/test-data/music"]}'
# Wait for job to complete (poll GET /api/jobs or watch logs for "album confidence")

# If barcode hit (REO Speedwagon with real MBZ): queue should be empty
curl -s "http://localhost:7474/api/music/queue?status=pending" | jq 'length'
# → 0 (auto-imported via barcode hit)

# If stubbed MBZ returns below-threshold results: queue entry exists with all 7 signals
QUEUE_ID=$(curl -s "http://localhost:7474/api/music/queue?status=pending" | jq -r '.[0].id')
curl -s "http://localhost:7474/api/music/queue/$QUEUE_ID" \
  | jq '.candidates[0] | {overallConfidence, signals}'
# → signals object with all seven numeric fields present and non-null

# Log verification
grep "album scan start" server.log
# → root=".../Hi Infidelity..." has_barcode=true has_isrcs=true

grep "album confidence" server.log
# → overall=<N> action=auto_import|queue

# If action=queue, investigate which signals fired:
grep "rg signal" server.log
grep "release signal" server.log
```

---

## Phase 7 — Import Services

### Task 15: Artist import — full enrichment on first create

**Goal**: On first import of a music artist: create all release groups, stubs for all releases under each RG, all member People with roles, all metadata keys from Task 10. Handle solo-artist Person-link.

**Files**:
- `internal/app/metadata/service.go` (update)
- `internal/app/metadata/service_test.go` (update)
- `internal/app/metadata/aggregator_integration_test.go` (update)

On `ImportEntry` for `KindArtist`:
1. Create all release groups (call `FetchEntryContent`)
2. For each RG: type-assert `ReleaseGroupContentSource`; create all release stubs
3. `FetchEntryPeople` → create/link People; solo: create Person from life-span metadata
4. Write all metadata keys to `LibraryEntry.Metadata`
5. Mark `IsDefault` release monitored when RG is monitored

**Logging requirements**:
- `INFO`: `"artist import" name=<name> mbid=<mbid> type=<group|person> rg_count=<N> release_stubs=<N> member_count=<N>`
- `INFO` per RG: `"release group created" rg_mbid=<mbid> title=<title> release_stubs=<N>`
- `INFO` per member: `"member linked" name=<name> role=<role> person_id=<id>`
- `WARN` when member already exists: `"member already exists" name=<name> person_id=<id>`

**Tests**:
- `TestImportEntry_Music_PopulatesAllMetadataKeys`
- `TestImportEntry_Music_CreatesReleaseGroupsWithAlbumType`
- `TestImportEntry_Music_CreatesReleaseStubs`
- `TestImportEntry_Music_SoloArtist_CreatesPerson`
- `TestImportEntry_Music_BandMembers_RolesPreserved`
- `TestImportEntry_Music_DefaultReleaseMonitored`
- Integration: `TestMetadataService_Integration_ImportREOSpeedwagon` — live MBZ; ≥5 members, ≥1 RG, ≥1 release stub

**Verified by curl**:
```bash
# Import REO Speedwagon via existing artist import endpoint
ENTRY_ID=$(curl -s -X POST http://localhost:7474/api/library/import \
  -H "Content-Type: application/json" \
  -d '{"source":"mbz","externalId":"bdc70372-7e8a-4cb9-8d33-f036b3b7cdc1"}' | jq -r '.id')

# Verify metadata keys
curl -s "http://localhost:7474/api/library/$ENTRY_ID" \
  | jq '.metadata | {artist_type, founded_date, aliases}'
# → artist_type must be "group", founded_date non-empty, aliases non-empty array

# Verify members
curl -s "http://localhost:7474/api/library/$ENTRY_ID" | jq '.people | length'
# → ≥5

# Verify release groups were created
curl -s "http://localhost:7474/api/library/$ENTRY_ID/groups" | jq 'length'
# → >0

# Pick Hi Infidelity and verify release stubs
HI_INF_ID=$(curl -s "http://localhost:7474/api/library/$ENTRY_ID/groups" \
  | jq -r '[.[] | select(.title | contains("Hi Infidelity"))] | .[0].id')

curl -s "http://localhost:7474/api/groups/$HI_INF_ID/releases" \
  | jq '[.[] | {title, status, isDefault, barcode}]'
# → ≥1 result; one with isDefault==true; all status=="stub" at this point
# → the 2024 release should show barcode="074646161425"

# Log verification
grep "artist import" server.log
# → name="REO Speedwagon" rg_count=<N> release_stubs=<N> member_count=<N>
```

---

### Task 16: Release import — tracks, MediaFile, tag write-back

**Goal**: When identified above threshold: create `MusicRelease` (or update stub → imported), create `Item` records for all tracks with correct `Metadata` keys, create `MediaFile` records with `SHA1` computed, write MBZ IDs back to file tags.

**Files**:
- `internal/app/music/import.go` (new) — `ReleaseImporter`
- `internal/app/music/import_test.go` (new)
- `internal/adapters/fs/music_tag_writer.go` (new)
- `internal/adapters/fs/music_tag_writer_test.go` (new)
- `internal/ports/filesystem.go` (update) — add `MusicTagWriter` interface

`MusicTagWriter.WriteIDs` writes only `MUSICBRAINZ_ALBUMID` and `MUSICBRAINZ_TRACKID`; all other tags unchanged; supports FLAC and MP3. Non-fatal — import continues on failure.

**Logging requirements**:
- `INFO`: `"release import start" release_mbid=<mbid> title=<title> track_count=<N>`
- `INFO`: `"release import complete" release_id=<id> tracks=<N> files=<N> status=imported`
- `DEBUG` per file: `"tag write-back" path=<path> release_mbid=<mbid> recording_mbid=<mbid>`
- `WARN` on tag write failure: `"tag write-back failed" path=<path> error=<err>` — import still continues

**Tests**:
- `TestReleaseImporter_CreatesAllEntities`
- `TestReleaseImporter_UpdatesStubToImported`
- `TestReleaseImporter_TracksHaveCorrectMetadataKeys`
- `TestReleaseImporter_MultiDisc_DiscNumbersCorrect`
- `TestReleaseImporter_CallsTagWriterPerTrack`
- `TestReleaseImporter_TagWriterFailure_DoesNotAbortImport`
- `TestMusicTagWriter_WritesAndReadsBack_FLAC` — copy test FLAC to temp; call `WriteIDs`; re-fingerprint; both MBZ tags present; no other tags changed

**Verified by curl** (full pipeline: scan → auto-import → verify):
```bash
# After scanning test-data/music/ with real MBZ adapter:

# 1. Queue is empty
curl -s "http://localhost:7474/api/music/queue?status=pending" | jq 'length'
# → 0

# 2. Find artist and release group
ENTRY_ID=$(curl -s "http://localhost:7474/api/library?contentType=music&kind=artist" \
  | jq -r '[.data[] | select(.name == "REO Speedwagon")] | .[0].id')

HI_INF_ID=$(curl -s "http://localhost:7474/api/library/$ENTRY_ID/groups" \
  | jq -r '[.[] | select(.title | contains("Hi Infidelity"))] | .[0].id')

# 3. Release is imported with correct barcode
RELEASE_ID=$(curl -s "http://localhost:7474/api/groups/$HI_INF_ID/releases" \
  | jq -r '[.[] | select(.status == "imported")] | .[0].id')

curl -s "http://localhost:7474/api/music/releases/$RELEASE_ID" \
  | jq '{status, barcode, trackCount}'
# → status=="imported", barcode=="074646161425", trackCount==10

# 4. Tracks have ISRCs, disc numbers, and release IDs
curl -s "http://localhost:7474/api/music/releases/$RELEASE_ID/tracks" \
  | jq '[.[0:3] | .[] | {title, isrc, discNumber, releaseId}]'
# → all three fields non-empty on each track

# 5. SHA1 on MediaFile
TRACK_ID=$(curl -s "http://localhost:7474/api/music/releases/$RELEASE_ID/tracks" | jq -r '.[0].id')
curl -s "http://localhost:7474/api/items/$TRACK_ID/file" | jq '.sha1'
# → non-empty hex string

# 6. Re-scan — no duplicates (re-scan shortcut must fire)
curl -s -X POST http://localhost:7474/api/commands/scan \
  -H "Content-Type: application/json" \
  -d '{"paths":["/path/to/test-data/music"]}'
# Wait, then:
curl -s "http://localhost:7474/api/music/releases/$RELEASE_ID/tracks" | jq 'length'
# → still 10

# Log verification
grep "release import complete" server.log
# → tracks=10 files=10 status=imported

grep "album re-scan shortcut" server.log   # (from second scan)
# → root=".../Hi Infidelity..."
```

---

### Task 17: AcoustID async job

**Goal**: After track import, queue a background job per track that computes AcoustID and writes it to `MediaFile.Metadata["acoustid"]`.

**Files**:
- `internal/domain/job.go` (update) — add `JobKindAcoustIDCompute`
- `internal/app/music/acoustid_job.go` (new)
- `internal/app/music/acoustid_job_test.go` (new)

Idempotent: skip if `Metadata["acoustid"]` already set. Non-fatal if `fpcalc` unavailable.

**Logging requirements**:
- `INFO`: `"acoustid job" media_file_id=<id> action=compute|skip_already_set`
- `INFO`: `"acoustid computed" media_file_id=<id>` — fingerprint is logged at DEBUG only (long string)
- `WARN`: `"acoustid failed" media_file_id=<id> error=<err>`

**Tests**:
- `TestAcoustIDJob_SkipsIfAlreadySet`
- `TestAcoustIDJob_WritesResult`
- `TestAcoustIDJob_FpcalcUnavailable_NonFatal`

**Verified by curl** (after import + jobs drain):
```bash
TRACK_ID=<from Task 16>
curl -s "http://localhost:7474/api/items/$TRACK_ID/file" | jq '.metadata.acoustid'
# → non-empty string if fpcalc installed; null if not — both acceptable
# null is NOT acceptable if fpcalc IS installed; verify with: which fpcalc
```

---

## Phase 8 — Remaining API Endpoints

### Task 18: API — remaining music endpoints

**Goal**: Queue import/dismiss flow, track music PATCH, missing list endpoints, and music fields on existing Group and Item responses.

**Files**:
- `internal/api/music_releases.go` (update) — add `GET /api/artists/{id}/releases`, `GET /api/music/releases/{id}/tracks`
- `internal/api/music_queue.go` (update) — add `POST /api/music/queue/{id}/import`
- `internal/api/items.go` (update) — add `PATCH /api/items/{id}/music`; include `releaseId`, `discNumber`, `isrc` in Item response
- `internal/api/groups.go` (update) — include `albumType` from `Metadata["album_type"]` in Group response
- `internal/api/types.go` (update) — `MusicTrackPatchRequest`

New routes:
```
GET    /api/artists/{id}/releases
GET    /api/music/releases/{id}/tracks
POST   /api/music/queue/{id}/import     body: {releaseMbid, rgMbid}
PATCH  /api/items/{id}/music            body: {releaseId?, groupId?, isrc?, discNumber?}
```

**Tests** (`internal/api/server_test.go`):
- `TestAPI_GetReleasesByArtist`
- `TestAPI_GetTracksByRelease`
- `TestAPI_PostQueueImport_ChangesStatusToMatched`
- `TestAPI_PatchTrackMusic_UpdatesISRC`
- `TestAPI_PatchTrackMusic_NilFieldsUntouched`
- `TestAPI_GetGroup_IncludesAlbumType`
- `TestAPI_GetItem_MusicFieldsPresent`

**Verified by curl**:
```bash
# Correct a track ISRC
curl -s -X PATCH http://localhost:7474/api/items/$TRACK_ID/music \
  -H "Content-Type: application/json" \
  -d '{"isrc":"XX0000000001"}' | jq '.isrc'
# → "XX0000000001"

# Verify group has albumType
curl -s http://localhost:7474/api/groups/$HI_INF_ID | jq '.albumType'
# → "studio"

# Verify item response includes all three music fields
curl -s http://localhost:7474/api/items/$TRACK_ID | jq '{isrc, discNumber, releaseId}'
# → all three non-empty
```

---

## Phase 9 — TypeScript Types and API Client

### Task 19: TypeScript types

**Goal**: Add all new music types to `web/src/types/index.ts`. Update existing types. TypeScript will catch API shape mismatches before the UI is built.

**Files**: `web/src/types/index.ts` (update)

New types: `ReleaseStatus`, `MusicRelease`, `MusicConfidenceSignals`, `MusicReleaseCandidate`, `MusicScanGroup`.

Updated types: `MediaFile` → add `sha1?: string`, `metadata?: Record<string, string>`; `Group` → add `albumType?: string`; `Item` → add `releaseId?: string`, `discNumber?: number`, `isrc?: string`.

**Verified by**:
```bash
npx tsc --noEmit
# → zero errors

# Smoke check: the following must compile cleanly:
# const s: MusicConfidenceSignals = { barcode:0, isrc:0, rgNameFuzzy:0, trackCount:0, trackTitleSet:0, duration:0, acoustid:0 }
# Removing any of the seven fields must produce a type error.
```

---

### Task 20: API client — music hooks

**Goal**: TanStack Query hooks for all music endpoints.

**Files**:
- `web/src/api/music.ts` (new)
- `web/src/api/music.test.ts` (new)

Functions: `getMusicRelease`, `listReleasesByGroup`, `listReleasesByArtist`, `getReleaseTrackList`, `createMusicRelease`, `patchMusicRelease`, `deleteMusicRelease`, `createMusicQueueItem`, `listMusicQueue`, `getMusicQueueItem`, `importMusicQueueItem`, `dismissMusicQueueItem`, `patchTrackMusic`.

**Tests** (`web/src/api/music.test.ts`):
- `getMusicRelease_returnsTypedRelease`
- `listMusicQueue_filtersByStatus`
- `getMusicQueueItem_signalsAllSevenPresent`
- `importMusicQueueItem_postsCorrectBody`
- `patchTrackMusic_sendsPartialPayload`

**Verified by**: `npx vitest run web/src/api/music.test.ts` — all pass.

---

## Phase 10 — UI

### Task 21: Artist detail — discography chips and metadata bar

**Goal**: Fix `albumSectionToken` to use `metadata.album_type` (replaces current `primary_type`/`secondary_types` read). Add metadata bar: founded date, location, artist type badge, external links.

**Files**: `web/src/pages/music/ArtistDetail.tsx` (update), `web/src/pages/music/ArtistDetail.test.ts` (new)

**Tests**: `AlbumSectionToken_StudioAlbum`, `AlbumSectionToken_LiveAlbum`, `AlbumSectionToken_Compilation`, `AlbumSectionToken_EPorSingle`, `ArtistMetadata_ShowsFoundedDate`, `ArtistMetadata_ShowsOfficialLink`, `ArtistMetadata_ShowsArtistTypeBadge`

**Verified by**: Navigate to REO Speedwagon artist page. "Hi Infidelity" appears under Albums chip. Metadata bar shows "Founded: 1967" and official site link. No albums appear under wrong chip.

---

### Task 22: Release Group detail — editions list

**Goal**: `AlbumDetail.tsx` shows Release Group at top + editions list with country flag, date, label, barcode, format, status badge, and monitored toggle per edition.

**Files**: `web/src/pages/music/AlbumDetail.tsx` (update), `web/src/pages/music/AlbumDetail.test.ts` (update)

**Tests**: `AlbumDetail_ShowsEditionList`, `AlbumDetail_ShowsStatusBadge`, `AlbumDetail_MonitoredToggle_CallsPatch`

**Verified by**: Navigate to "Hi Infidelity". Edition list shows the 2024 Digital as "imported" and the 1980 original as "stub". Clicking the stub's monitor toggle sends a PATCH (visible in devtools network tab) and the UI updates optimistically.

---

### Task 23: Track listing — ISRC, disc badge, file status

**Goal**: Track rows show ISRC label, disc badge when `discNumber > 1`, and "wanted" for tracks with no file.

**Files**: `web/src/components/media/ItemCard.tsx` (update), `web/src/components/media/ItemCard.test.ts` (update), `web/src/pages/music/AlbumDetail.tsx` (update)

**Tests**: `ItemCard_ShowsISRC`, `ItemCard_ShowsDiscBadge_WhenGT1`, `ItemCard_NoDiscBadge_SingleDisc`

**Verified by**: Navigate to the 2024 imported edition of "Hi Infidelity". All 10 tracks show their ISRCs. No disc badge appears. A stub release with no files shows tracks in "wanted" state.

---

### Task 24: Music import queue page

**Goal**: Settings page showing pending `MusicScanGroup` entries with a `ConfidenceSignalTable` showing all seven signals per entry.

**Files**:
- `web/src/pages/settings/ImportQueuePage.tsx` (update) — add music queue tab
- `web/src/components/scan/MusicQueueCard.tsx` (new)
- `web/src/components/scan/MusicQueueCard.test.ts` (new)
- `web/src/components/scan/ConfidenceSignalTable.tsx` (new)
- `web/src/components/scan/ConfidenceSignalTable.test.ts` (new)

`ConfidenceSignalTable` props: `signals: MusicConfidenceSignals`. Seven rows with name, score bar, percentage. Zero-score rows render muted.

**Tests**: `MusicQueueCard_ShowsFolderPath`, `MusicQueueCard_ShowsTrackCount`, `ConfidenceSignalTable_ShowsAllSevenSignals`, `ConfidenceSignalTable_ZeroMuted`, `ConfidenceSignalTable_PercentageFormatted`

**Verified by**: With a below-threshold queue entry present, navigate to Settings → Import Queue → Music tab. Card shows folder path, track count, candidate name and confidence %. Signal table shows all 7 rows; "Barcode: 0%" is muted; "Duration: 88%" has a coloured bar.

---

### Task 25: Music queue manual resolution dialog

**Goal**: "Import" button on a queue card opens a 3-step dialog: Artist → Release Group → Release → Confirm.

**Files**: `web/src/components/scan/MusicImportDialog.tsx` (new), `web/src/components/scan/MusicImportDialog.test.ts` (new)

Uses `StepIndicator` and `EditDrawer` (existing). Does not duplicate step UI from `ImportDialog`.

**Tests**: `MusicImportDialog_PreFillsTopCandidate`, `MusicImportDialog_CanSearchDifferentArtist`, `MusicImportDialog_Step2_ShowsReleaseGroups`, `MusicImportDialog_Step3_ShowsEditionDetails`, `MusicImportDialog_Confirm_CallsImportEndpoint`

**Verified by**: Open a queue entry's "Import" dialog. Select a different artist in step 1 — step 2 RG selection clears. Step 3 shows country flag and barcode. Click "Import" — fires `POST /api/music/queue/{id}/import` and dialog closes.

---

### Task 26: Track metadata correction editor

**Goal**: Edit button on a track row opens a drawer with ISRC, disc number, Release, and Release Group fields.

**Files**: `web/src/components/edit/editors/TrackMusicEditor.tsx` (new), `web/src/components/edit/editors/TrackMusicEditor.test.ts` (new)

Calls `patchTrackMusic(itemId, patch)` on save.

**Tests**: `TrackMusicEditor_ShowsCurrentISRC`, `TrackMusicEditor_Save_CallsPatch`, `TrackMusicEditor_ReleaseDropdown_ListsCurrentGroupReleases`

**Verified by**: Navigate to a track. Click edit. Change ISRC to `"XX0000000001"`. Save. Track row shows new ISRC. Network tab shows one PATCH to `/api/items/{id}/music`.

---

## Phase 11 — Monitoring

### Task 27: Release Group monitoring

**Goal**: When a Group is monitored, the `MusicRelease` with `isDefault == true` is automatically set `Monitored = true`. Additional releases opt in via the editions list toggle.

**Files**: `internal/app/library/service.go` (update), `internal/app/library/service_test.go` (update)

**Logging requirements**:
- `INFO`: `"rg monitored" group_id=<id> default_release_id=<id>`
- `INFO`: `"rg monitored no default" group_id=<id>` — no default release; monitoring still succeeds

**Tests**: `TestMonitorReleaseGroup_SetsDefaultReleaseMonitored`, `TestMonitorReleaseGroup_NonDefaultUnaffected`, `TestMonitorReleaseGroup_NoDefault_NoOp`

**Verified by curl**:
```bash
curl -s -X PATCH http://localhost:7474/api/groups/$HI_INF_ID \
  -H "Content-Type: application/json" -d '{"monitored":true}'

curl -s "http://localhost:7474/api/groups/$HI_INF_ID/releases" \
  | jq '[.[] | {title, monitored, isDefault}]'
# → only the entry with isDefault==true has monitored==true
```

---

## Phase 12 — End-to-End Validation

### Task 28: E2E — REO Speedwagon full flow (barcode path)

**Files**: `internal/app/scan/e2e_music_test.go` (new)

`TestMusicE2E_REOSpeedwagon_FullFlow` — real FLACs, stubbed MBZ returning fixture data matching the known release. Asserts after one scan pass:

1. `MusicScanGroupRepository.List(pending)` → 0
2. "REO Speedwagon" artist entry present; `Metadata["artist_type"] == "group"`; `Metadata["founded_date"]` non-empty; aliases non-empty
3. ≥5 members; at least one vocalist, one guitarist
4. "Hi Infidelity" group with `Metadata["album_type"] == "studio"`
5. ≥1 `MusicRelease` with `Barcode == "0074646161425"` and `Status == imported`
6. 10 items; each has non-empty `isrc`, `disc_number == "1"`, `release_id` set
7. Each item's `MediaFile` has non-empty `SHA1`
8. Re-scan → item count still 10; queue still empty

**Verified by**:
```bash
go test -run TestMusicE2E_REOSpeedwagon_FullFlow ./internal/app/scan/... -v
```
Test output must log each of the 8 assertion results clearly.

---

### Task 29: E2E — worst-case identification (no barcode, no ISRCs, no MBZ IDs)

**Files**: `internal/app/scan/e2e_music_worstcase_test.go` (new)

`TestMusicE2E_WorstCase_FuzzyPipelineAutoImports` — synthesised files, tags contain only `artist`, `album`, `track_total`, `disc_total`, `date`. Stubbed MBZ returns "The Enchanted Works of Stevie Nicks" fixture.

Asserts: `signals.Barcode == 0`, `signals.ISRC == 0`, `signals.RGNameFuzzy >= 0.50`, `signals.TrackCount == 0.20`, `signals.TrackTitleSet > 0`, `OverallConfidence >= threshold`, queue empty.

**Verified by**:
```bash
go test -run TestMusicE2E_WorstCase_FuzzyPipelineAutoImports ./internal/app/scan/... -v
```
Output must print the full `MusicConfidenceSignals` struct showing which signals fired.

---

## Dependency Graph

```
Tasks 1–5   Foundation (domain → ports → contracts → storage)
             └─ everything depends on these

Task 6      Minimal API CRUD ← 1–5
             └─ DESIGN VERIFICATION GATE — verify before Task 7

Task 7      Fingerprinter ← 1 only (can start same day Task 1 merges)

Tasks 8–9   Scan grouper + tag summary ← 1–2, 7

Task 10     MBZ artist enrichment ← 1–2  (parallel to 8–9)
Task 11     MBZ releases + barcode + ISRC ← 1–2  (parallel to 8–9)

Tasks 12–13 Confidence cascade signals ← 1–2, 9, 11

Task 14     Album identifier assembly ← 6, 8–13

Task 15     Artist import ← 6, 10, 11
Task 16     Release import + tag write-back ← 6, 14, 15
Task 17     AcoustID job ← 4, 16

Task 18     Remaining API endpoints ← 6, 15, 16

Tasks 19–20 TypeScript types + hooks ← 6, 18

Tasks 21–26 UI pages ← 19–20 (all parallel to each other)

Task 27     Monitoring ← 15, 16, 22

Tasks 28–29 E2E ← everything
```
