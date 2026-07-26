# Sidecar Classification and Cover Art (M10)

Music's implementation of [0024](../adr/0024-pipeline-core.md)'s
sidecar-classification-before-identification decision, plus the cover-art
attachment step that extends
[pipeline-music-persist.md](pipeline-music-persist.md) (M9). See
[music-pipeline-milestones.md](music-pipeline-milestones.md) for where M10
sits in the build order.

## Scope

**In scope:** the classification capability that keeps non-audio files out
of the scan pipeline entirely, and the cover-art attachment step at
persist time.

**Explicitly out of scope:** any non-image sidecar getting routed
somewhere. `.nfo`/`.cue`/`.log`/`.m3u` files have no destination in
Purser's domain model — classification excludes them from processing,
full stop, nothing further ever happens to them.

## Classification runs in `ScanService.Trigger`, before Tasks exist

Unlike grouping (M3), classifying one file needs no sibling visibility —
it's a per-path check (extension, filename convention), so it doesn't need
`Grouping`'s batch shape. It runs in `Trigger`, which already resolves
content type there (M3's design) to write `job.Params["content_type"]`:
the walked file list is filtered by the registered `SidecarClassifier`
*before* `taskLabels` is built. A file classified as a sidecar never
becomes a `Task` at all — no wasted hash computation, no per-file `Step`
to account for, and a user watching job progress never sees a
`cover.jpg` entry that goes nowhere.

```
SidecarKind string
const (
    SidecarKindNone  SidecarKind = ""       // not a sidecar — proceeds through the pipeline normally
    SidecarKindImage SidecarKind = "image"  // cover-art candidate — excluded from Tasks, handled at persist
    SidecarKindOther SidecarKind = "other"  // .nfo/.cue/.log/etc — excluded from Tasks, ignored entirely
)

SidecarClassifier interface {
    ContentTypes() []domain.ContentType
    Classify(ctx context.Context, path string) (SidecarKind, error)
}
```

Default (`NoopClassifier`, unregistered content types): always
`SidecarKindNone` — every file proceeds normally, same "no cost to opt
out" treatment every other default capability in this build gets.

## Correction to M3: grouping's own filtering is now redundant

Grouping used to filter non-audio files out of its own sibling-pattern
check, so a stray `cover.jpg` next to `CD1`/`CD2` wouldn't break the
multi-disc roll-up. Now that classification runs *upstream*, in `Trigger`,
grouping never sees a non-audio path at all — its copy of the same
extension check has been removed from
[pipeline-grouping-capability.md](pipeline-grouping-capability.md). One
authoritative classification, not two places defining "is this audio"
that could quietly drift apart.

## Music's classification rules

- **Audio** (`SidecarKindNone`): known audio extensions (`.flac`, `.mp3`,
  `.m4a`, `.ogg`, `.wav`, `.aac`, `.wma`, `.opus`, ...) — this list is now
  the single source of truth M3's grouping also implicitly relies on.
- **Image** (`SidecarKindImage`): image extensions (`.jpg`, `.jpeg`,
  `.png`, `.webp`, `.gif`) — filename convention isn't checked at
  classification time (any image file in scope of the scan qualifies);
  filename matters later, at attachment time, for *ranking* which image
  is the primary cover.
- **Other** (`SidecarKindOther`): everything else — `.nfo`, `.cue`, `.log`,
  `.m3u`, and anything unrecognized.

## Cover art: found fresh at persist time, not tracked between scan and persist

No new entity or repository is needed to "remember" a classified image
file from scan time until a `MusicRelease` exists to own it. `GroupKey` is
already a folder path, and it's still sitting there when M9's `Persist`
runs — so `Persist`, after resolving the `MusicRelease` (M9 step 5), does
one plain directory listing of the group's folder for image files, ranked
by filename convention: `cover.*` highest, `folder.*` next, `front.*`/
`album.*` after that, any other image file last. If a group is dismissed
and never persisted, nothing ever looks at its images — correct, since
there's no release to attach them to.

**All matched images get attached, not just the top-ranked one.**
`domain.Image` already has a `Priority int` field for exactly this — each
match's rank becomes its `Priority`, and existing priority-based display
ordering sorts out which one shows first. No need to pick a single winner
during classification.

## First real caller of `ImageStore`'s write path

[0013](../adr/0013-image-blob-storage.md) already noted the local-disk
`ImageStore` adapter exists and is tested but nothing calls it — no owner
type can receive uploaded bytes yet. M10 closes that gap for Music:

```
key, err := imageStore.Put(ctx, "music_release", release.ID, fileReader)
// then:
imageRepo.Create(ctx, &domain.Image{
    ID:        domain.NewID(),
    OwnerType: "music_release",
    OwnerID:   release.ID,
    ImageType: domain.ImageTypePoster, // closest existing generalized value to "primary cover" — a proposal, not firmly asserted
    URL:       urlFrom(key),           // per 0013's existing key→URL convention
    Priority:  rank,
    Source:    "local_scan",
})
```

## Duplicate-attachment safeguard — simpler than 0026's mechanism

Re-running `Persist` on a retry (per M9's idempotent-recovery model) would
re-attach the same cover art as duplicate `Image` rows, since images have
no natural uniqueness key the way an MBID does. Before running the
cover-art step, check `ImageRepository.List(ownerType, ownerID)` for the
release; if it already has any images attached, skip the step entirely. A
plain existence check, not a full reservation-document mechanism — a
duplicate `Image` row is a mess to clean up, not a correctness bug on the
scale [0019](../adr/0019-tag-identity-and-get-or-create.md)/[0026](../adr/0026-external-id-get-or-create.md)
exist to prevent, so it doesn't need their machinery.

## What this deliberately doesn't do

- No recursion into disc subfolders for per-medium art (e.g. `cdart`) —
  only the group's top-level folder (`GroupKey`) is checked. Worth
  revisiting if real box sets turn out to need it.
- No provider-fetched cover art (TheAudioDB, fanart.tv) — this milestone
  only covers art already sitting on disk next to the audio files.
