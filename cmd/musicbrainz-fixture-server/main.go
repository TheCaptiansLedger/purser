// Command musicbrainz-fixture-server is a standalone HTTP server serving
// internal/adapters/musicbrainz/fixtureserver's fixed, fictitious
// MusicBrainz dataset — CI-only tooling, never shipped in a release build
// (deliberately excluded from .goreleaser.yaml, per
// docs/adr/0017-build-and-release-goreleaser.md). Makefile's
// _k6-app-start builds and runs this alongside purser serve, pointing
// PURSER_MUSICBRAINZ_BASE_URL at it, so k6's flow suite can exercise the
// Music Persister's real MusicBrainz-calling code path without a live
// network dependency. See docs/technical/pipeline-music-persist.md.
package main

import (
	"log"
	"net/http"
	"os"
	"purser/internal/adapters/musicbrainz/fixtureserver"
	"time"
)

func main() {
	addr := os.Getenv("MUSICBRAINZ_FIXTURE_ADDR")
	if addr == "" {
		addr = "127.0.0.1:18080"
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           fixtureserver.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("musicbrainz-fixture-server listening on %s", addr) //nolint:gosec // CI-only tooling binary; addr is either a fixed default or an operator-set env var on the CI runner, never untrusted input
	log.Fatal(srv.ListenAndServe())
}
