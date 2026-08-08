package config

import (
	"fmt"
	"purser/internal/domain"
	"purser/pkg/nametemplate"
	"text/template"
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

	// Organize configures the generic Organizer (docs/adr/0024-pipeline-core.md,
	// docs/technical/pipeline-music-organizer.md), keyed by domain.ContentType
	// so each content type can organize into an entirely different library
	// tree with an entirely different naming convention rather than sharing
	// one global root/template.
	Organize map[domain.ContentType]OrganizeConfig `mapstructure:"organize"`

	// AutoOrganize gates whether a content type's Persister calls the
	// Organizer automatically immediately after creating a MediaFile.
	// Regardless of this setting, organizing is always available as an
	// explicit, user-triggered RPC — auto-organize being off never removes
	// the manual path. See docs/technical/pipeline-music-organizer.md.
	AutoOrganize bool `mapstructure:"auto_organize"`
}

// OrganizeConfig pairs the base directory a content type organizes into
// with the relative Go text/template naming pattern joined onto it.
type OrganizeConfig struct {
	// Root is the base directory this content type organizes into.
	Root string `mapstructure:"root"`

	// Template is a relative Go text/template pattern, joined onto Root,
	// rendered against whatever map the content type's TemplateDataBuilder
	// produces.
	Template string `mapstructure:"template"`
}

// DefaultPipeline returns Pipeline's defaults: both extra hashes off, no
// watched roots, confidence threshold at its starting value of 0.75, no
// organize configuration (organizing a content type with no entry here is
// a configuration error surfaced by the Organizer itself, not a silent
// no-op), and AutoOrganize off — organizing only ever runs when an operator
// opts in, per its own doc comment.
func DefaultPipeline() Pipeline {
	return Pipeline{
		EnableMD5:           false,
		EnableSHA512:        false,
		ScanRoots:           []ScanRoot{},
		ConfidenceThreshold: 0.75,
		Organize:            map[domain.ContentType]OrganizeConfig{},
		AutoOrganize:        false,
	}
}

// Validate checks that every configured ScanRoot carries both a Path and a
// ContentType — a root missing either can never be resolved to a Grouping
// implementation, so it fails fast at startup rather than silently falling
// back to IdentityGrouping for every file under it. Every configured
// OrganizeConfig entry must carry a non-empty Root and a Template that
// parses as valid Go text/template syntax — a broken naming template
// should fail at startup, not on the first file an operator tries to
// organize.
func (p Pipeline) Validate() error {
	for _, r := range p.ScanRoots {
		if r.Path == "" {
			return fmt.Errorf("pipeline: scan_roots entry missing path")
		}
		if r.ContentType == "" {
			return fmt.Errorf("pipeline: scan_roots entry %q missing content_type", r.Path)
		}
	}

	for contentType, oc := range p.Organize {
		if oc.Root == "" {
			return fmt.Errorf("pipeline: organize entry %q missing root", contentType)
		}
		if oc.Template == "" {
			return fmt.Errorf("pipeline: organize entry %q missing template", contentType)
		}
		if _, err := template.New("organize").Funcs(nametemplate.Funcs()).Parse(oc.Template); err != nil {
			return fmt.Errorf("pipeline: organize entry %q has an invalid template: %w", contentType, err)
		}
	}
	return nil
}
