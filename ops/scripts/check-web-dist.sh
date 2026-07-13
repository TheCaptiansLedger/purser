#!/usr/bin/env bash
# Run by goreleaser's before.hooks (.goreleaser.yaml) and the Makefile's
# build target — web/dist must exist before any Go build or docker build
# runs, per docs/adr/0017-build-and-release-goreleaser.md.
set -euo pipefail

if [ ! -d web/dist ]; then
  echo "web/dist missing; run: cd web && npm ci && npm run build" >&2
  exit 1
fi
