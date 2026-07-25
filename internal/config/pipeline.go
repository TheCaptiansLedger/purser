package config

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
	ScanRoots []string `mapstructure:"scan_roots"`
}

// DefaultPipeline returns Pipeline's defaults: both extra hashes off, no
// watched roots.
func DefaultPipeline() Pipeline {
	return Pipeline{
		EnableMD5:    false,
		EnableSHA512: false,
		ScanRoots:    []string{},
	}
}

// Validate is a no-op today — every boolean combination is valid. Present
// for symmetry with every other component, per docs/adr/0010-configuration.md,
// so a future field with a real invariant isn't a breaking addition to
// Config.Validate's call chain.
func (p Pipeline) Validate() error {
	return nil
}
