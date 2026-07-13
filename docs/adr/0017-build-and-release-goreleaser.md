# 0017. Build & Release: Goreleaser as the Build Model

Status: Accepted

## Context

Purser needs to ship a native binary for Windows, Linux, and macOS (Intel
and Apple Silicon), plus container images for `linux/amd64` and
`linux/arm64`. `.github/workflows/release.yml` currently hand-rolls this:
`semantic-release` cuts the version tag and changelog, a `build` job runs
a single `go build` (linux/amd64 only — no other OS/arch is produced at
all), and a separate `container` job runs `docker/build-push-action` with
Buildx/QEMU by hand for the multi-arch image. Both the `build` and `pr.yml`
jobs already reference `-ldflags="-X purser/internal/version.Version=..."`
and run `./bin/purser --version` as a smoke test — `internal/version`
doesn't exist yet and `cmd/purser`'s root command never sets Cobra's
`Version` field, so that step has been silently broken since it was
written.

Cross-compiling five OS/arch combinations and building/pushing multi-arch
manifests by hand in shell steps is exactly the problem goreleaser exists
to solve, and it can drive both the binary matrix and the container image
build from one config file instead of two separately-maintained code
paths.

## Decision

- **`internal/version`** is a new leaf package holding `Version`, `Commit`,
  and `Date` (build-time `-X` injected, `"dev"`/`"unknown"` defaults for
  unstamped `go build`/`go run`). `cmd/purser/root.go` sets `cmd.Version`
  on the Cobra root command from it, which gives `--version` for free via
  Cobra's built-in flag — fixing the CI smoke test that already assumed
  this existed.
- **Goreleaser (`.goreleaser.yaml`) is the only place that builds release
  artifacts.** It owns:
  - The binary matrix: `windows/amd64`, `linux/amd64`, `linux/arm64`,
    `darwin/amd64`, `darwin/arm64`, each with the same ldflags injecting
    `internal/version.{Version,Commit,Date}`.
  - Archives and checksums for the GitHub Release.
  - Container images for `linux/amd64` and `linux/arm64` via its
    `dockers`/`docker_manifests` config, built from `ops/Containerfile`.
- **`ops/Containerfile` is copy-only.** It no longer runs `go build`
  inside a builder stage — goreleaser hands it an already-built binary
  for the target arch (plus the pre-built `web/dist`, per
  [0009](0009-cli-stack.md)'s "build web before build go" ordering,
  carried into the release pipeline the same way `ops/Containerfile`'s
  comment already documented for the old manual flow). This keeps exactly
  one thing responsible for "how is the Go binary built" instead of the
  Containerfile and goreleaser each doing their own `go build` with
  ldflags that can drift apart.
- **`semantic-release` still owns versioning.** It creates the git tag,
  the changelog, and the (initially asset-less) GitHub Release exactly as
  today. Goreleaser runs *after*, against that tag, with
  `release.mode: append` so it publishes binaries/checksums/images into
  the release semantic-release already created rather than creating a
  competing one.
- **The local dev loop does not run the full release pipeline on every
  rebuild.** `make compose build` uses `goreleaser build --single-target
  --snapshot --clean` to produce just the current-platform binary, then a
  plain `docker build -f ops/Containerfile .` for one local-arch image —
  goreleaser is still the thing that built the binary (same config, same
  ldflags), but the multi-arch `dockers`/`docker_manifests` fan-out is
  release-only, gated behind CI, so iterating locally doesn't pay for
  QEMU emulation on every change.
- **CI**: `release.yml`'s hand-rolled `build` and `container` jobs are
  replaced by a single `goreleaser release --clean` job that runs after
  `semantic-release` and only when it published a new version — same
  trigger condition as today's `container` job.

## Consequences

- One config file (`.goreleaser.yaml`) is the source of truth for every
  release artifact's build flags, instead of ldflags living in three
  places (`Makefile`, `ops/Containerfile` build args, and CI YAML) that
  can silently drift.
- Adding a new target OS/arch is a one-line change to `.goreleaser.yaml`,
  not a new matrix entry hand-wired into a GitHub Actions job.
- `ops/Containerfile` can no longer be `docker build`ed standalone without
  first producing a binary at the expected path — `make compose build`
  and CI both handle this, but a contributor running raw `docker build`
  against it directly will get a confusing "binary not found" failure
  instead of a working image. This tradeoff is accepted because it keeps
  binary-building in exactly one place.
- Goreleaser becomes a required local tool (already pinned/installed the
  same way `buf`/`k6` are per the `tools` Makefile target) for anyone
  testing a release build before pushing.

## Self-Audit Checklist

1. Does any code path build a release binary or container image without
   going through goreleaser (a hand-rolled `go build` in CI, a Containerfile
   builder stage doing its own `go build`)? If yes — fix it; goreleaser is
   the only build path for release artifacts.
2. Do the ldflags injected by goreleaser, `make compose build`'s snapshot
   build, and any lingering CI step all point at the same
   `internal/version` symbols? If they've drifted, fix it.
3. Does `release.yml` still let `semantic-release` create the tag/release
   before goreleaser runs, with goreleaser appending to it rather than
   creating a second release for the same tag? If no — fix it.
4. Is the local `make compose build` loop still fast (single-target,
   snapshot, no multi-arch QEMU fan-out)? If someone made it always build
   all five OS/arch targets locally, that's a usability regression — fix
   it.
