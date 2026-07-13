# 0013. Image Blob Storage: A Local-Filesystem-First `ImageStore` Port

Status: Accepted

## Context

`ports.ImageRepository` (see [0012](0012-datastore-persistence.md))
persists only the `domain.Image` metadata row — id, owner, type,
dimensions, a plain `URL string`. Nothing in the codebase writes actual
image bytes anywhere; `URL` is just a field. Storing images in the
document datastore alongside metadata was explicitly rejected before that
work even started — "saving them in a database is crazy" was the original
framing — so a separate mechanism for the bytes themselves has always
been the intended second half of image support.

A pre-reset version of this codebase had exactly this: `internal/adapters/fs/image_downloader.go`
and `internal/adapters/fs/paths.go`, deleted at the `chore!: reset
codebase` commit. That code fetched a remote URL over HTTP and wrote it
to a sharded directory (`{base}/{entityType}/{id[0:2]}/{id}{ext}`) with
atomic temp-file-then-rename writes — added specifically because an
earlier, non-atomic version left partial files on failure
(`fix(adapter): use atomic temp+rename write in ImageDownloader.Download`).
That design assumed one image per entity; the current `domain.Image` is
explicitly polymorphic and supports multiple images per owner
(`OwnerType`/`OwnerID`, each with its own `Image.ID`), so the sharding key
changes from the owner's ID to the image's own ID, but the rest of the
design — sharded directories, atomic writes — is proven and worth
keeping.

A generic HTTP response cache already exists
(`pkg/httpclient`'s caching transport, wrapping `pkg/cache.Cache`) but is
the wrong tool for this: its only implementation
(`pkg/cache/memory`) defaults to a 64 MiB *total* in-memory bound shared
across every cached HTTP response in the process, a 15-minute TTL, and is
wiped on `Close()` — none of which fits "permanently mirror a
multi-gigabyte image library locally." Fetching a remote image and
persisting a local copy is a distinct future concern layered on top of
this ADR's `ImageStore`, not solved by it or by that cache.

## Decision

### `ImageStore` is a `ports` interface, not an adapter-internal type

Unlike `datastore.Datastore` ([0012](0012-datastore-persistence.md)),
which only other adapters ever see, `ImageStore` will be depended on
directly by a future service (an image upload/import flow: write bytes
via `ImageStore`, then persist the resulting reference via
`ImageRepository`). Per [0001](0001-hexagonal-architecture.md), that
means it belongs in `internal/ports`, with its own contract-test suite
(`internal/ports/imagestoretest`), same convention as every other port in
this codebase:

```go
type ImageStore interface {
    Put(ctx context.Context, ownerType, id string, r io.Reader) (key string, err error)
    Get(ctx context.Context, key string) (io.ReadCloser, error)
    Delete(ctx context.Context, key string) error
}
```

It stays deliberately narrow and separate from `ImageRepository` — blob
bytes and metadata rows are different capabilities with different
reasons to change, the same ISP reasoning
[0012](0012-datastore-persistence.md) already applied to `Datastore`
versus the entity repos.

`Put` returns an opaque `key` rather than requiring the caller to
construct one: the file extension is decided by the store itself (via
content-type sniffing, not a caller-supplied hint — see below), so the
caller cannot know the full key in advance. `Get`/`Delete` operate on
exactly the key `Put` returned; a caller never builds one by hand.

### The adapter decides the extension, not the caller

`Put` sniffs the first 512 bytes via `http.DetectContentType` (stdlib,
already imported everywhere `net/http` is used, no new dependency) rather
than trusting a caller-supplied `Content-Type` header or file extension.
This is deliberate: a caller passing a raw `io.Reader` (an uploaded
multipart file, bytes freshly downloaded from a URL, anything) often
doesn't have a trustworthy content type at all, and sniffing centralizes
"how do we pick `.jpg` vs `.png`" in one place instead of every future
caller reimplementing it. The known-type-to-extension mapping (jpeg, png,
webp, gif, svg; unknown defaults to `.jpg`) is the same one the pre-reset
`ImageDownloader` used.

### Layout, atomicity, and limits — carried forward from the pre-reset design, adjusted for multi-image-per-owner

- **Sharded path:** `{root}/{ownerType}/{shard(id)}/{id}{ext}`, `shard` =
  the first 2 characters of `id`. Keyed on the *image's own* ID rather
  than the owner's ID (the pre-reset scheme's key), because an owner can
  now have more than one image.
- **Atomic writes:** every `Put` writes to a temp file in the target
  shard directory, then `os.Rename`s it into place — carrying forward the
  fix the pre-reset code needed after shipping a non-atomic version first.
  This adapter starts with the fix already applied.
- **Size limit:** `Put` wraps its reader in `io.LimitReader(r, maxBytes+1)`
  and rejects (no partial file left behind) anything that hits the cap —
  a bare `io.Copy` from an arbitrary caller-supplied reader with no bound
  is an unbounded-write/resource-exhaustion risk this ADR closes off from
  the start rather than patching in later.
- **Path-traversal defense on read:** `Get`/`Delete` resolve `key` to an
  absolute path and explicitly verify it still has `root` as a prefix
  before touching the filesystem. `Put` never generates a `key` containing
  `..`, but `key` round-trips through `domain.Image.URL` — a plain
  string, no format enforced by the type system — once a future service
  starts persisting it, so this is defense in depth against a
  malformed/tampered value reaching `Get`, not a defense against `Put`
  itself.

### Configuration

`internal/config.Media{Path string}` backs the `media.path` /
`PURSER_MEDIA_PATH` key that was already documented in
`ops/purser.yaml`/`.env.example` before this ADR (and before
[0012](0012-datastore-persistence.md)'s work) but never wired to
anything. An empty `Path` derives `<Paths.DataDir>/media` in
`Config.DefaultConfig`, the same cross-component derivation pattern
`Database.Badger.DataDir` already uses — explicit override still wins.

### Not built in this pass

- **Wiring into `cmd/purser`.** No RPC or service exists yet that would
  call `ImageStore` — constructing the local adapter in the composition
  root with nothing calling it would just be inert code. The upload/import
  service is the natural trigger to wire it in.
- **An S3 (or other object-store) adapter.** The port is designed to make
  one trivial (`io.Reader`/`io.ReadCloser`, no filesystem-specific types
  leak into the interface) but none is built yet.
- **A remote-image fetcher** (HTTP GET → `ImageStore.Put`). This is the
  actual answer to "cache a remote image locally," but it's a thin,
  separate piece layered on `ImageStore`, not part of the store itself.

## Consequences

- Blob storage is swappable (local now, S3 later) behind one port, same
  substitutability guarantee every other port in this codebase already
  has, proven by a shared contract test rather than by inspection.
- Nothing is reachable from the running server yet — this ADR closes a
  capability gap, not a user-facing feature. The upload/import service is
  required before any of this does anything observable.
- `ImageStore` and `ImageRepository` are two separate writes with no
  transactional relationship between them — a caller that writes bytes
  via `ImageStore.Put` and then fails before calling
  `ImageRepository.Create` leaves an orphaned file with nothing pointing
  at it. There is no cleanup mechanism for this yet; the future
  upload/import service is responsible for ordering these calls sanely
  (write bytes first, only persist metadata on success) and, eventually,
  for reconciling orphans — not solved here.

## Self-Audit Checklist

1. Does any code outside `internal/adapters/imagestore/**` construct a
   `key` string by hand instead of using the value `Put` returned? If
   yes — fix it; `key` is opaque by design.
2. Does `Put` (or any future adapter implementing this port) trust a
   caller-supplied content type/extension instead of sniffing? If yes —
   fix it.
3. Does any write path use a bare `io.Copy`/`os.WriteFile` instead of the
   temp-file-plus-rename pattern? If yes — fix it; that's exactly the bug
   class the pre-reset code hit once already.
4. Does `Get`/`Delete` (or a future adapter) skip verifying the resolved
   path stays under `root`? If yes — fix it.
5. Does anything call `ImageStore` and `ImageRepository` in the wrong
   order (metadata persisted before bytes are confirmed written)? If
   yes — fix it; bytes must exist before the row that points at them does.
