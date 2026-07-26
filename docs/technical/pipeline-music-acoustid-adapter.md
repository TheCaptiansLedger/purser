# AcoustID Adapter (M5)

Design for the AcoustID client adapter backing Music identification
([0025](../adr/0025-music-identification-confidence-scoring.md)), the
same category of work as
[music-musicbrainz-adapter.md](music-musicbrainz-adapter.md) (M2). See
[music-pipeline-milestones.md](music-pipeline-milestones.md) for where M5
sits in the build order.

## Scope

**In scope:** the adapter — computing a Chromaprint fingerprint locally
and looking it up against the AcoustID API.

**Explicitly out of scope:** deciding *when* to call it. Per
[0025](../adr/0025-music-identification-confidence-scoring.md), AcoustID
only runs when tag-derived signals didn't already resolve a group —
that's M7's gate, not built here. Also out of scope: deciding whether a
result agrees or contradicts a tag-derived candidate — that's M8's
judgment call. M5 hands back what AcoustID said, nothing more.

## Two separate operations, not one combined call

```
AcoustIDClient interface {
    Fingerprint(ctx context.Context, path string) (fingerprint string, durationSeconds float64, err error)
    Lookup(ctx context.Context, fingerprint string, durationSeconds float64) ([]AcoustIDMatch, error)
}
```

- **`Fingerprint`** — local only, no network. Shells out to `fpcalc`,
  parses its output. Kept separate from `Lookup` so the fingerprint value
  can be stored (`MediaFile.Metadata`, per [0021](../adr/0021-music-domain-model.md)'s
  original note that this should happen regardless of whether a real
  lookup adapter exists) even on a path that never calls the API.
- **`Lookup`** — network call to `api.acoustid.org`. Uses `pkg/httpclient`
  + `pkg/cache` the same way the MusicBrainz adapter does (M2) — no
  separate infrastructure decision here, same reuse requirement applies.

**Neither runs unconditionally.** Computing a Chromaprint fingerprint means
decoding the whole file — real CPU cost even with zero network involved.
Both methods only get called by M7, and only for a group whose tag-derived
signals weren't enough — the same "don't pay for work you don't need"
principle the hash-based short-circuit already applies elsewhere in this
pipeline ([0024](../adr/0024-pipeline-core.md)).

## Rate limiting: unverified, needs a real number before trusting it

MusicBrainz's 1 req/sec is documented policy; nothing in this project's
research so far pins down AcoustID's actual limit. The adapter gets the
same shape of client-side limiter M2 has, with a conservative placeholder
default — flagged as unverified and tunable, not asserted correct, same
treatment the confidence score bands get elsewhere in this doc set.

## User-Agent and errors

Same fixed `"Purser/" + version.Version` convention as M2 — no config key.
A no-match result from AcoustID (empty `results`, or an unknown
fingerprint) maps to `ports.ErrNotFound`, not a special AcoustID-specific
error type.

## Response shape: provider-faithful, not pre-collapsed

```
AcoustIDMatch struct {
    AcoustID   string  // AcoustID's own UUID for this fingerprint cluster
    Score      float64 // 0.0–1.0, AcoustID's own confidence
    Recordings []AcoustIDRecording
}

AcoustIDRecording struct {
    MBID          string
    Title         string
    ReleaseGroups []AcoustIDReleaseGroup
}

AcoustIDReleaseGroup struct {
    MBID  string
    Title string
    Type  string
}
```

Mirrors AcoustID's actual nested response (a fingerprint can match several
recordings, each appearing on several release groups) rather than
flattening to a single ID list — flattening would silently throw away the
recording-to-release-group association M8 needs to check whether a match
actually corroborates the specific candidate being scored.

## Duration: computed twice, deliberately not shared

`ffprobe` (M4) and `fpcalc` (M5) each report their own duration for a file
when both run. `Lookup` uses `fpcalc`'s self-reported value, not M4's
`ffprobe` value passed in — avoids a precision mismatch between two
different tools' measurements. Accepted as a small redundant cost rather
than engineered around, since this path only runs for the subset of files
where tags weren't enough.

## Deployment dependency

`fpcalc` (part of Chromaprint) joins `ffprobe` (M4) as a required binary
wherever AcoustID matching is enabled — both belong in the same media-
toolchain build/deployment note, not two disconnected footnotes.

## Testing

Per [0003](../adr/0003-go-testing-standards.md), split by method:

- **`Fingerprint`** is local-only (shells out to `fpcalc`, no network) —
  not subject to the live-network rule at all. Its default test runs the
  real `fpcalc` binary against a small checked-in sample audio file; no
  fixture needed, since there's no external service involved.
- **`Lookup`** calls the real AcoustID API — same rule as the MusicBrainz
  adapter: the default suite runs against recorded response fixtures, CI
  never touches the network, and a `live`-build-tag-gated verification
  test is the only thing allowed to call `api.acoustid.org` for real, run
  manually. M7/M8/M9's tests depend on the `AcoustIDClient` port and use a
  fake — they never construct this adapter.
