package config

import (
	"fmt"
	"purser/internal/domain"
)

// ScanRoot pairs a watched/scannable directory with the domain.ContentType
// it holds. Grouping/fingerprinting/identifying a file can't pick a
// content-type-specific capability without knowing its root's content
// type, which a flat directory string alone can't carry — see
// docs/technical/pipeline-grouping-capability.md.
type ScanRoot struct {
	// Path is the directory watched/scanned.
	Path string `mapstructure:"path"`

	// ContentType is the domain.ContentType every file under Path belongs
	// to.
	ContentType domain.ContentType `mapstructure:"content_type"`
}

// Pipeline configures the common scan/hash/queue pipeline shared across
// content types (docs/adr/0024-pipeline-core.md). OSHash and SHA1 are
// always computed by the pipeline; MD5 and SHA512 are opt-in extra
// full-file hashes some downstream integrations still key off — gated
// here since hashing every discovered file two extra times is a real I/O
// cost most installs don't need.
type Pipeline struct {
	// EnableMD5 turns on MD5 hashing of every discovered file, in addition
	// to the always-computed OSHash/SHA1.
	EnableMD5 bool `mapstructure:"enable_md5"`

	// EnableSHA512 turns on SHA512 hashing of every discovered file, in
	// addition to the always-computed OSHash/SHA1.
	EnableSHA512 bool `mapstructure:"enable_sha512"`

	// ScanRoots are the directories a live pkg/fswatch.Watcher watches for
	// changes, each debounced unit triggering the same ScanService.Trigger
	// path an explicit TriggerScan RPC call does — see
	// docs/adr/0024-pipeline-core.md's "Discovery" section. Empty means no
	// watcher is started at all: no cost to opt out.
	ScanRoots []ScanRoot `mapstructure:"scan_roots"`

	// ConfidenceThreshold is the auto-import cutoff the shared
	// DecisionService compares a group's top domain.MatchCandidate.Score
	// against (docs/adr/0024-pipeline-core.md's "decide" stage): at or
	// above it, the candidate is auto-persisted; below it, the group stays
	// in the UnmatchedFile review queue. 0.75 is a starting number carried
	// from the fuzzy tier's score band in
	// docs/technical/music-identification.md — unvalidated, not a claim
	// it's correct.
	ConfidenceThreshold float64 `mapstructure:"confidence_threshold"`
}

// DefaultPipeline returns Pipeline's defaults: both extra hashes off, no
// watched roots, confidence threshold at its starting value of 0.75.
func DefaultPipeline() Pipeline {
	return Pipeline{
		EnableMD5:           false,
		EnableSHA512:        false,
		ScanRoots:           []ScanRoot{},
		ConfidenceThreshold: 0.75,
	}
}

// Validate checks that every configured ScanRoot carries both a Path and a
// ContentType — a root missing either can never be resolved to a Grouping
// implementation, so it fails fast at startup rather than silently falling
// back to IdentityGrouping for every file under it.
func (p Pipeline) Validate() error {
	for _, r := range p.ScanRoots {
		if r.Path == "" {
			return fmt.Errorf("pipeline: scan_roots entry missing path")
		}
		if r.ContentType == "" {
			return fmt.Errorf("pipeline: scan_roots entry %q missing content_type", r.Path)
		}
	}
	return nil
}
