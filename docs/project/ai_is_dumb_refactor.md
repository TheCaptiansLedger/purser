# AI_IS_DUMB Refactor

Fixes every structural problem identified in the June 2026 codebase audit so that the next content-type adapter (TMDB, TVDB, etc.) can be written in isolation, tested completely, and debugged from logs alone.

## Execution Order

| Issue | Branch | Kind | Depends On |
|---|---|---|---|
| 1 | `chore/image-handler-logging` | logging | nothing |
| 2 | `chore/image-downloader-port` | refactor + tests | nothing |
| 3 | `chore/consolidate-fs-adapter` | refactor + tests | 2 |
| 4 | `test/image-handler-coverage` | tests | 3 |
| 5 | `refactor/aggregator-fanout` | refactor + tests | nothing |
| 6 | `test/metadata-shared-stubs` | tests | 2 |
| 7 | `chore/structured-logging` | logging | 2, 3 |

Issues 1 and 5 are fully independent and can be done at any time. Issues 2 → 3 → 4 are a strict chain. Issue 6 should follow Issue 2. Issue 7 closes out after 2 and 3.

---

## Issue 1 — Add debug logging to the image handler

**Branch:** `chore/image-handler-logging`

**Problem:** When artwork is missing there is zero log output. No way to know what file path the server looked for without reading source. Every debugging session turns into source-diving.

**Changes — `internal/api/images.go`:**

- At the top of `get`, before the candidate loop, log `entityType` and `entityID` at Debug level.
- Inside the loop, log each candidate path checked.
- On a successful `os.Stat`: `slog.Debug("image.served", "path", candidate)`.
- On fall-through to 404: `slog.Debug("image.not_found", "entity_type", entityType, "entity_id", entityID)`.

**Tests:** None — pure observability change.

---

## Issue 2 — Implement the `ImageDownloader` port and `fs` adapter

**Branch:** `chore/image-downloader-port`

**Problem:** `fetchImage` in `app/metadata/service.go` uses `http.DefaultClient` directly inside an application service. It is untestable (all service tests pass `mediaPath: ""` to bypass it entirely), has no timeout, and violates the hexagonal dependency rule. Every future content-type adapter inherits this untestability.

**Changes:**

### `internal/ports/image.go` (new file)

```go
type ImageDownloader interface {
    // Download fetches the image at url and writes it under entityType/entityID
    // in the configured media directory. Returns the file extension (e.g. ".jpg")
    // on success, "" on any error (errors are logged internally).
    Download(ctx context.Context, url, entityType, entityID string) string
}
```

### `internal/adapters/fs/image_downloader.go` (new file)

- Struct holds `mediaPath string` and `client *http.Client` (injected, not `http.DefaultClient`).
- Constructor sets `Timeout: 30 * time.Second` on the client.
- Move the body of `service.fetchImage` here verbatim, then clean up.
- Move `extFromContentType` here.

### `internal/app/metadata/service.go`

- Replace `mediaPath string` field and `fetchImage` method with `downloader ports.ImageDownloader`.
- All call sites: `s.downloader.Download(ctx, url, entityType, entityID)`.
- Remove `import "net/http"`, `import "io"`, `import "os"` from service.go.
- `New(...)` signature: swap `mediaPath string` for `downloader ports.ImageDownloader`. Pass `nil` to disable image fetching (existing behaviour for tests).

### `cmd/purser/main.go`

- Construct `fsadapter.NewImageDownloader(cfg.Media.Path)` and inject into `metadata.New`.

**Tests — `internal/adapters/fs/image_downloader_test.go`:**

- `TestImageDownloader_HappyPath`: httptest server serving a tiny JPEG; verify file on disk at expected sharded path with `.jpg` extension.
- `TestImageDownloader_HTTP500`: verify returns `""`, no file created.
- `TestImageDownloader_NetworkError`: verify returns `""`.
- `TestImageDownloader_BadContentType`: verify defaults to `.jpg`.
- `TestImageDownloader_PathTraversal`: verify written file stays inside `mediaPath`.
- `TestExtFromContentType`: table test covering jpeg, png, webp, gif, svg, unknown.

**Tests — `internal/app/metadata/service_test.go`:**

- Inject a `stubImageDownloader` (returns `".jpg"` always).
- Add `TestRefreshStudio_FetchesImageForItemsWithURL`: verify `Download` is called for items with `ImageURL` set.
- Add `TestRefreshStudio_SkipsImageForItemsWithoutURL`: verify `Download` is not called when `ImageURL` is empty.
- Add `TestRefreshStudio_ImageDownloaderFailure`: downloader returns `""`, verify refresh still completes and item is saved with empty `CoverPath`.

---

## Issue 3 — Consolidate `internal/media` into `internal/adapters/fs/`; fix path duplication

**Branch:** `chore/consolidate-fs-adapter`

**Problem:** `internal/media/images.go` is an unauthorized package that does not exist in the hexagonal layer map. `internal/adapters/fs/` is an empty directory. The shard calculation is duplicated between `media/images.go` (`shard()` function) and `api/images.go` (inline on lines 37–40). Changing the sharding scheme requires two edits and will still break because neither site is tested.

**Changes:**

1. Create `internal/adapters/fs/paths.go`. Move `ImagePath`, `EnsureDirs`, `MigrateFlat`, `shard` from `internal/media/images.go` here. Package name: `fs`.

2. In `internal/api/images.go`, replace the inline shard + path join block with `fs.ImagePath(h.basePath, entityType, entityID, ext)` inside the extension loop. Import `purser/internal/adapters/fs`.

3. Delete `internal/media/images.go` and the `internal/media/` directory.

4. Update `cmd/purser/main.go`: replace `media.EnsureDirs` / `media.MigrateFlat` calls with `fs.EnsureDirs` / `fs.MigrateFlat`.

**Tests — `internal/adapters/fs/paths_test.go`:**

- `TestImagePath_ShardedLayout`: `ImagePath("/base", "entries", "abc123", ".jpg")` → `/base/entries/ab/abc123.jpg`.
- `TestImagePath_ShortID`: IDs shorter than 2 chars do not panic.
- `TestImagePath_NoExt`: empty extension produces path without trailing dot.
- `TestEnsureDirs_CreatesSubdirs`: verify all four entity subdirs exist after call.
- `TestMigrateFlat_MovesFiles`: create a flat temp layout; call `MigrateFlat`; verify files are in sharded subdirs.
- `TestMigrateFlat_Idempotent`: second call produces no changes and no error.
- `TestMigrateFlat_MissingDirIsSkipped`: non-existent entity dir is silently skipped.

---

## Issue 4 — Add image serving endpoint tests

**Branch:** `test/image-handler-coverage`

**Problem:** `GET /api/v1/images/{entityType}/{entityID}` has zero test coverage, including the path traversal protection. This is security-adjacent untested code.

**Changes — `internal/api/server_test.go`:**

Update `newHandlerWithDB` (or add a `newHandlerWithMedia` variant) to accept a `mediaPath string` so tests can write real files and verify they are served.

Tests to add:

- `TestImages_Get_NotFound`: request a valid entity type with a non-existent ID → 404.
- `TestImages_Get_InvalidEntityType`: entity type not in the allowlist → 404.
- `TestImages_Get_PathTraversal_EntityID`: entityID containing `..` or `/` → 404.
- `TestImages_Get_ServesJPEG`: write a small file to the expected sharded path; verify 200 and `Content-Type: image/jpeg`.
- `TestImages_Get_ServesPNG`: same with `.png` extension.
- `TestImages_Get_MultipleExtensions`: only a `.png` exists; verify it is served even though handler iterates `.jpg` first.

---

## Issue 5 — Move fan-out into the Aggregator; remove duplication from Service

> **Hexagonal violation to fix here:** `Service.fetchAlbumCovers` calls `s.sourceByName("fanart")` to pull the fanart adapter out by name, bypassing the port abstraction. The service should not know about a specific adapter. Fix this as part of the fan-out refactor — either route album-cover fetching through the aggregator or introduce a typed port.

**Branch:** `refactor/aggregator-fanout`

**Problem:** `Service.SearchStudios` and `Service.SearchPeople` each implement the same `WaitGroup + channel` goroutine fan-out pattern that already exists in the Aggregator. The Aggregator was built to own fan-out. Having it in both places means any fix must be applied twice.

**Changes:**

1. Add `SearchStudios(ctx context.Context, query string, limit int) ([]*domain.ExternalStudio, error)` to `Aggregator`. Move the fan-out body from `Service.SearchStudios` here.

2. Add `SearchPeople(ctx context.Context, query string, limit int) ([]*domain.ExternalPerson, error)` to `Aggregator`. Same.

3. Rewrite `Service.SearchStudios` to filter sources by content type, then delegate:
   ```go
   return s.agg.SearchStudios(ctx, query, limit)
   ```

4. Rewrite `Service.SearchPeople` identically.

**Tests — `internal/app/metadata/aggregator_test.go`:**

- `TestAggregator_SearchStudios_FanOut`: two sources; verify both called; results merged.
- `TestAggregator_SearchStudios_SourceError`: one source errors; verify other source's results still returned.
- `TestAggregator_SearchStudios_NoMatchingSources`: empty sources slice → empty result, no error.
- `TestAggregator_SearchPeople_FanOut`: mirrors SearchStudios tests.
- `TestAggregator_SearchPeople_SourceError`: mirrors SearchStudios error test.

---

## Issue 6 — Shared test stubs for the `app/metadata` package

**Branch:** `test/metadata-shared-stubs`

**Problem:** `service_test.go` defines eight stub types from scratch. Any new test file in this package rewrites them. `aggregator_test.go` contains overlapping stubs. The `stubImageDownloader` needed after Issue 2 would otherwise be duplicated too.

**Changes:**

1. Create `internal/app/metadata/stubs_test.go` (package `metadata_test`). Move all stub types from `service_test.go` here:
   - `stubEntryRepo`
   - `stubItemRepo`
   - `stubGroupRepo`
   - `stubPersonRepo`
   - `stubTagRepo`
   - `stubExternalIDRepo`
   - `seededExternalIDRepo`
   - `seededArtistExternalIDRepo`
   - `stubJobQueue`
   - `stubSource`
   - `stubMusicSource`

2. Add `stubImageDownloader` here:
   ```go
   type stubImageDownloader struct {
       ext   string // returned on every call; "" to simulate failure
       calls []string // URLs passed to Download
   }
   func (d *stubImageDownloader) Download(_ context.Context, url, _, _ string) string {
       d.calls = append(d.calls, url)
       return d.ext
   }
   ```

3. Remove duplicate definitions from `service_test.go` and `aggregator_test.go`.

4. Update `service_test.go` helper `newService()` and `refreshSvc()` to inject a `&stubImageDownloader{ext: ".jpg"}`.

**Tests:** This issue adds no new test cases. The existing tests must pass without change after the refactor.

---

## Issue 7 — Fill remaining logging gaps

**Branch:** `chore/structured-logging`

**Problem:** Several execution paths produce no log output. Incomplete refresh pipelines, failed network imports, and successful image downloads are all invisible.

**Changes:**

### `internal/app/metadata/service.go`

- Top of `RefreshStudio`, after loading the entry:
  ```go
  slog.Info("studio.refresh.start", "entry_id", entryID, "name", entry.Name, "source", src.Name())
  ```
- Top of `RefreshArtist`, after loading the entry and resolving source:
  ```go
  slog.Info("artist.refresh.start", "entry_id", entryID, "name", entry.Name, "source", src.Name())
  ```
- In `importOrFindNetwork`, on successful parent resolution: `slog.Debug("network.resolved", "parent_id", parentID, "created", network != nil)`.
- In `importOrFindNetwork`, on save error: `slog.Error("network.create.failed", "name", req.ParentName, "error", err)` before returning.

### `internal/adapters/fs/image_downloader.go` (from Issue 2)

- On successful write, before returning `ext`:
  ```go
  slog.Debug("image.downloaded", "url", url, "entity_type", entityType, "entity_id", entityID, "ext", ext)
  ```

**Tests:** None — logging changes are observable only at runtime.
