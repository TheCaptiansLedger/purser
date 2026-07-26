# Organizer: Naming Template for Music (M11)

Music's implementation of [0024](../adr/0024-pipeline-core.md)'s
Organizer — rename + move, user-configurable, always available manually.
Called automatically from
[pipeline-music-persist.md](pipeline-music-persist.md) (M9) when enabled.
See [music-pipeline-milestones.md](music-pipeline-milestones.md) for where
M11 sits in the build order.

## Scope

**In scope:** the generic render-and-move mechanics, Music's
`TemplateDataBuilder`, the auto/manual trigger wiring, and the config
shape.

**Explicitly out of scope:** anything content-type-specific beyond
Music's own template data — the generic `Organizer` mechanics are shared,
but this milestone only builds Music's data-gathering side of that split.

## Split: generic mechanics, content-type-specific data

Same shape as every other capability in this build (`ContentTypes()`
fan-out registry):

```
TemplateDataBuilder interface {
    ContentTypes() []domain.ContentType
    BuildTemplateData(ctx context.Context, item *domain.Item) (map[string]any, error)
}
```

The generic `Organizer` (shared, not Music-owned): given a `MediaFile`,
fetches its `Item`, dispatches to the registered `TemplateDataBuilder` for
the `Item`'s content type, renders that content type's configured Go
`text/template` against the resulting data map, computes the destination
path, moves the file, and updates `MediaFile.Path`. It has no idea what
`ArtistName` or `AlbumTitle` mean — it just renders a template against
whatever map it's handed.

## Music's `TemplateDataBuilder`

Building a useful naming template needs more than the `Item` alone —
cross-entity lookups, the same shape M9's `Persist` already does:

- `Item` itself: `Title` → `TrackTitle`, `Sequence` → `TrackNumber`,
  `Metadata["disc_number"]` → `DiscNumber`.
- `Group` (via `Item.GroupID`): `Title` → `AlbumTitle`.
- `MusicRelease` (via `Item.Metadata["release_id"]`): `Date` → `Year`,
  `MediumCount` → `DiscCount` (drives whether the template includes a
  disc-number prefix at all).
- `LibraryEntry`/Artist (via `Group.LibraryEntryID`): `Name` →
  `ArtistName`.
- `Ext`: the file's current extension (`filepath.Ext` on its existing
  `Path`), not `MediaFile.Container` — `Container` isn't reliably
  populated by M9's `Persist` today, and reading the extension directly
  off the actual file is simpler and more robust either way.

## Two triggers, same shape as M9's `Persister`

- **Automatic**: M9's `Persist`, after successfully creating a
  `MediaFile`, calls the `Organizer` immediately if
  `config.Pipeline.AutoOrganize` is on.
- **Manual**: a new `OrganizerService` with `Organize(ctx, mediaFileID)
  (*domain.MediaFile, error)` — callable regardless of the toggle's
  current setting. Per [0024](../adr/0024-pipeline-core.md), auto-organize
  being off must never remove this path, so it's a first-class RPC, not a
  debug escape hatch.

## Move mechanics: `Rename` first, safe fallback for cross-filesystem moves

`os.Rename` fails outright when source and destination are on different
filesystems (a common real case — source volume vs. library volume). The
Organizer tries `Rename` first; on that specific failure, it falls back to
copy-then-verify-then-delete-original. **The original is never deleted
before the copy is confirmed complete** — a partial copy followed by
deleting the source is a real data-loss path, not an acceptable shortcut.

## Collision handling: refuse, don't overwrite

If the computed destination already has a file at it (a template bug, or
a leftover from an interrupted previous organize), silently overwriting
risks destroying an unrelated file. The Organizer errors out instead — on
the manual RPC path, that error surfaces directly to the caller; on the
automatic path (from M9), it's logged and the file is simply left where it
is, still reachable via its current `MediaFile.Path` — a naming collision
never blocks the overall persist operation from succeeding.

## Config shape: per content type, not one global root/template

```
config.Pipeline.Organize map[string]OrganizeConfig  // keyed by content type

OrganizeConfig struct {
    Root     string // base directory this content type organizes into
    Template string // relative Go text/template, joined onto Root
}
```

Per-content-type pairing (not a single global root/template) so Music and
any future content type can organize into entirely different library
trees with entirely different naming conventions.

A starting default for Music, not asserted as the right convention:

```
{{.ArtistName}}/{{.AlbumTitle}}{{if .Year}} ({{.Year}}){{end}}/{{if gt .DiscCount 1}}{{.DiscNumber}}-{{end}}{{printf "%02d" .TrackNumber}} - {{.TrackTitle}}{{.Ext}}
```

## "Never triggers a reimport" — already guaranteed, no new work needed here

The Organizer updates `MediaFile.Path` as part of the move. Even if
`pkg/fswatch` independently fires on the new location, the hash-based
"already known" short-circuit (built well before this milestone) already
recognizes the file by hash and just updates its `Path` again rather than
re-queueing it. This falls out of mechanisms M1/M4/M9 already established
— M11 doesn't need to build anything new to keep this guarantee, only
avoid breaking it (i.e., always update `Path` as part of the move, never
skip that step).
