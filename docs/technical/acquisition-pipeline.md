# Acquisition Pipeline

Describes the end-to-end flow for both managed acquisition (Purser-initiated)
and unmanaged acquisition (user brings a file from outside Purser).

---

## Indexer Integrations

Purser uses external indexers (disk search, prowlarr, etc) for its aggregation layer.
It does not implement indexers itself.  In general, these applications do not need
to know about Purser, there is no app registration or integration from the indexer. 
The indexers adapter will call the indexers API directly using an API just as JSON/HTTPS.

---

## Managed Acquisition (Purser-initiated)

The user has marked an item as wanted and Purser drives the full pipeline.

### 1. Search

Purser queries known indexers search API with parameters derived from the wanted
item's metadata. Release matching and scoring determine which result to grab
(details TBD). If no suitable release is found, the item stays in `wanted`.

### 2. Grab

Purser dispatches the selected release to the configured clients
(torrent, nzb, file search). The item transitions to `grabbed` and Purser records
the torrent hash for tracking.

### 3. Download Tracking

Purser polls the download client's API on a regular interval to check torrent
status. Optionally, the download client can be configured to call a Purser
webhook on completion for near-instant notification. Polling is the reliable
baseline; webhooks are an optimization.

### 4. Post-Download Import

When the download client reports completion:

1. Compute the file's OSHash and PHash.
2. Attempt a fingerprint lookup against StashDB (and other configured providers).

**High confidence** — fingerprint matches a known scene: auto-import, rename,
move, tag. Item transitions to `imported` with `match_confidence: verified`.

**Medium confidence** — fingerprint not found in any provider, but the item was
grabbed by searching for a specific scene: import, rename, move, tag. Item
transitions to `imported` with `match_confidence: name_matched`. The scene
record already exists in Purser's database from the original metadata discovery
step; the file is linked to it without cryptographic confirmation.

**Low confidence** — release name is ambiguous or does not parse cleanly against
the expected item: add to the unmatched file queue for manual resolution.

A missing fingerprint match is not an error. StashDB may not have every encoding
of a known scene indexed. `match_confidence` records the distinction so users can
spot-check if needed.

---

## Unmanaged Acquisition (User-supplied file)

The user acquires a file outside Purser and either drops it in a watched folder
or triggers an explicit scan.

### 1. Fingerprint Lookup

Purser computes the file's OSHash and PHash and queries configured metadata
providers.

**Match found:** rename, move, tag, and link to the matching scene. No further
action needed.

**No match:** add to the unmatched file queue.

### 2. Unmatched File Queue

The unmatched file queue is a first-class feature exposed in the UI, not an
error log. Each entry holds:

- File path and computed fingerprints
- Discovery timestamp
- Any low-confidence candidates Purser identified (by file size, duration, or
  partial name match)

The user resolves each entry by one of two paths:

**Scene exists in Purser's database** — user searches the local database, selects
the matching scene, and confirms. Purser renames, moves, and tags the file using
the existing scene record. `match_confidence` is set to `manual`.

**Scene is not in Purser's database** — user first triggers a metadata fetch
(search StashDB/TPDB by title, performers, etc.) to add the scene, then matches
the file to it as above.

Where Purser has low-confidence candidates, the UI surfaces them so the user can
confirm rather than search from scratch.

---

## Item State Machine

```
wanted → grabbed → imported
```

`imported` carries a `match_confidence` field:

| Value          | Meaning                                              |
|----------------|------------------------------------------------------|
| `verified`     | Fingerprint confirmed by a metadata provider         |
| `name_matched` | Grabbed for a specific scene; fingerprint not found  |
| `manual`       | User explicitly matched the file in the UI           |

---

## Download Notification Strategy

Notification comes from the download client:

- **Polling (required):** Purser queries the download client API on a configurable
  interval. Survives misconfigured clients.
- **Webhooks (optional):** Download clients that support run-on-completion hooks
  (e.g. qBittorrent's "run external program") can call a Purser endpoint for
  immediate notification.
