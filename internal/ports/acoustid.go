package ports

import "context"

// AcoustIDClient is the port for the single AcoustID adapter
// (docs/technical/pipeline-music-acoustid-adapter.md, ADR-0025). Like
// MusicBrainzClient, this is a deliberate exception to "ports describe
// capability, never a specific provider"
// (docs/adr/0001-hexagonal-architecture.md): AcoustID is a singular
// acoustic-fingerprint identity source with exactly one real
// implementation. Its methods return AcoustID's own DTO shapes below, not
// domain types — mapping a match into a domain.Artist/Group/MusicRelease
// is the caller's job (the Music scan identifier, M7), not this port's.
//
// Fingerprint and Lookup are deliberately separate operations, not one
// combined call: Fingerprint is local-only (no network) so the fingerprint
// value can be persisted even on a path that never calls the API; Lookup
// is the network call. Deciding when either method runs is explicitly out
// of scope here — that's M7's gate.
type AcoustIDClient interface {
	// Fingerprint computes a Chromaprint fingerprint for the audio file at
	// path by shelling out to fpcalc. Local only — no network call.
	Fingerprint(ctx context.Context, path string) (fingerprint string, durationSeconds float64, err error)

	// Lookup resolves a previously computed fingerprint/duration pair
	// against the AcoustID API. Returns ErrNotFound when AcoustID reports
	// no match (empty results, or an unknown fingerprint) — not a
	// AcoustID-specific error type.
	Lookup(ctx context.Context, fingerprint string, durationSeconds float64) ([]AcoustIDMatch, error)
}

// AcoustIDReleaseGroup is one release group AcoustIDRecording appears on,
// as reported by AcoustID.
type AcoustIDReleaseGroup struct {
	MBID  string `json:"id"`
	Title string `json:"title"`
	Type  string `json:"type"`
}

// AcoustIDRecording is one MusicBrainz recording a fingerprint cluster
// matched, as reported by AcoustID. A single recording can appear on
// several release groups.
type AcoustIDRecording struct {
	MBID          string                 `json:"id"`
	Title         string                 `json:"title"`
	ReleaseGroups []AcoustIDReleaseGroup `json:"releasegroups"`
}

// AcoustIDMatch is one fingerprint-cluster match AcoustID returned for a
// Lookup call. A single fingerprint can match several recordings — this
// shape mirrors AcoustID's actual nested response rather than flattening
// to a single ID list, since the recording-to-release-group association
// is what M8 needs to check whether a match corroborates the specific
// candidate being scored.
type AcoustIDMatch struct {
	AcoustID   string              `json:"id"`
	Score      float64             `json:"score"`
	Recordings []AcoustIDRecording `json:"recordings"`
}
