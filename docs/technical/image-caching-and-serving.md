# Image Caching, Upload, and Serving

Closes the gap [0013](../adr/0013-image-blob-storage.md) named and
deliberately deferred: `ports.ImageFetcher`, `ports.ImageStore`, and
`ports.ImageRepository` all exist and are contract-tested, but nothing in
the codebase today ever calls them in sequence, and nothing serves stored
bytes back out. `ImageService` (`internal/service/image.go`) is plain
`domain.Image` metadata CRUD — `Image.URL` is just a string field it never
inspects — so a provider-cached image and a raw hotlinked remote URL look
identical to it today.

Cross-module infrastructure — People photos, Music artist/album art,
AfterDark scene images all use exactly this, none of it Music-specific.
Written now because [music-web-ui.md](music-web-ui.md) is the first real
caller.

## Scope

**In scope:** two new blob-primitive RPCs, the byte-serving HTTP route,
the client-side call sequence for both the provider-cache and user-upload
flows, and orphan-write handling.

**Explicitly out of scope:** an S3/object-store adapter (already deferred
by 0013); per-owner-type policy for which `image_type`s exist or where the
"choose an image" affordance appears in any given module's UI (that's each
module's own doc — see `music-web-ui.md`'s Artwork section); thumbnail/
resized-variant generation (`ImageStore` keeps exactly one copy per
`Image` row; revisit only if a real need for multiple sizes shows up).

## Decision

### Two new primitive RPCs — no server-side sequencing

A new `ImageBlobService`, wrapping `ImageStore` (and, for one RPC,
`ImageFetcher`) — **never `ImageRepository`**. That exclusion is the whole
point: composing "get bytes onto disk" with "create the metadata row" is
the caller's job, not this service's. Folding both into one call would
recreate the God-endpoint shape this project doesn't build — the UI/CLI is
the composition path, calling narrow APIs in the right order, not a new
RPC that does the ordering for it server-side.

```
rpc CacheRemoteImage(CacheRemoteImageRequest) returns (CacheRemoteImageResponse)
rpc UploadImage(UploadImageRequest) returns (UploadImageResponse)
```

`CacheRemoteImageRequest{url}` → `ImageFetcher.Fetch(url)` piped straight
into `ImageStore.Put`. `UploadImageRequest{data: bytes}` → `ImageStore.Put`
only, no fetch. Both return the same shape:

```
message ImageBlobResponse {
  string key = 1;          // opaque — see ImageStore.Put's own contract
  int32 width = 2;
  int32 height = 3;
  string content_type = 4;
  int64 size_bytes = 5;
}
```

`width`/`height`/`content_type` are sniffed server-side (decoding the
image header), never trusted from a client-supplied hint — same reasoning
0013 already applies to file-extension sniffing. Returning them here
means the caller can pass them straight into `CreateImage`'s
`Image.Width`/`Height` without re-deriving them.

### Composition: client calls the existing `CreateImage` RPC itself

`ImageService.CreateImage` is unchanged — no new fields, no new
parameters. The caller (web UI or CLI) does two RPC calls in sequence:

1. `ImageBlobService.CacheRemoteImage(url)` **or**
   `ImageBlobService.UploadImage(bytes)` → `{key, width, height, ...}`.
2. `ImageService.CreateImage(Image{url: key, owner_type, owner_id,
   image_type, source, width, height, priority})`.

Symmetric from step 2 on regardless of which blob RPC supplied the key —
"which image is this owner's poster" code never needs to branch on
provider-vs-upload. This also gives the UI a natural pause point between
steps 1 and 2 (preview a fetched provider image, let the user confirm or
pick a different one, before committing it as the owner's art) without a
separate "cancel" RPC — an uncommitted key with no `Image` row is just an
orphan, handled below.

### Orphan writes: an accepted, bounded cost, not solved here

0013 already names this risk (`Put` succeeds, `Create` never happens →
orphaned file, no transactional link between the two writes). Splitting
composition out to the client doesn't introduce a new failure mode, it
just makes an existing one more visible: a closed browser tab mid-flow, or
a client that fetches a key and never calls `CreateImage`, is the same
shape 0013 already accepted as unsolved. No cleanup job ships in this
pass. If disk usage from orphans becomes a real problem, the fix is a
later sweep (diff `ImageStore` keys against `ImageRepository` rows,
reclaim anything unreferenced past some age) — named here so it isn't
rediscovered as a surprise, not built now.

### Byte serving: a plain HTTP handler, not a Connect RPC

```
GET /media/images/{id}
```

Looks up `domain.Image` by `id` via `ImageRepository.Get`, reads bytes via
`ImageStore.Get(img.URL)`, writes them back with the sniffed
`Content-Type` and a long-lived `Cache-Control` — images are immutable
once keyed (a changed image gets a new `Image` row and a new `ImageStore`
key, never an in-place overwrite, consistent with `Put`'s own "opaque key"
contract), so aggressive caching is safe.

Registered on the same `http.ServeMux` `newServeMux` (`cmd/purser/serve.go`)
already builds, alongside the Connect handlers and the SPA's static-asset
handler — not a new pattern. It has to be a plain handler: a browser
`<img src>` (or the lightbox) needs a cacheable `GET`, and a Connect unary
RPC is a `POST` with a body, not something an `<img>` tag can point at.

### Full sequence, both flows

**Provider-cache** (e.g. "use this fanart.tv poster"):
1. `FanartTVService.LookupArtist`/`TheAudioDBService.LookupArtist` already
   returns raw provider URLs — unchanged, no new work there.
2. User picks one in the UI.
3. `ImageBlobService.CacheRemoteImage(url)` → `key`.
4. `ImageService.CreateImage(Image{url: key, source: "fanart.tv", ...})`.
5. UI renders `GET /media/images/{new Image.ID}`.

**User upload:**
1. User picks a local file.
2. `ImageBlobService.UploadImage(bytes)` → `key`.
3. `ImageService.CreateImage(Image{url: key, source: "user", ...})`.

### Lightbox

Clicking any rendered image opens it full-screen (pre-reset precedent:
issue #288 — restated here, not reinvented). Reuses `GET
/media/images/{id}` directly, the same bytes the thumbnail already
rendered — no separate hi-res endpoint. `ImageLightbox` is added to
[the style guide's Component vocabulary](../design/style-guide.md#component-vocabulary),
shared across every module, same as `Card`/`Hero`/`StatusBadge`.

## Consequences

- `ImageService` stays untouched — still exactly `PersonService`-shaped
  CRUD, per 0011's SRP-per-entity rule.
- Every module's "attach an image" flow is two RPC calls done in the UI,
  visible in the client code, not one call whose internal ordering has to
  be trusted.
- Orphaned blob files are a known, accepted gap until a real reclaim need
  shows up — not a silent omission.

## Self-Audit Checklist

1. Does `ImageBlobService` (or any future caller) call
   `ImageRepository` directly, instead of leaving `CreateImage` to the
   client? If yes — fix it; that's exactly the composition split this doc
   exists to keep out of the server.
2. Does anything trust a client-supplied `width`/`height`/`content_type`
   instead of the server's own sniff? If yes — fix it.
3. Does the byte-serving handler skip setting a long-lived
   `Cache-Control`, or serve a key without resolving it through
   `ImageRepository` first (i.e. serving arbitrary `ImageStore` keys with
   no ownership check)? If yes — fix it.
4. Does any code assume `Image.URL` is a fetchable remote URL rather than
   an opaque local `ImageStore` key once `CreateImage` has been called? If
   yes — fix it; post this doc, every persisted `Image.URL` is a local key.
