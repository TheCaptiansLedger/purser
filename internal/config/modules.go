package config

// ModuleConfig gates one content-type module: whether it's active at all,
// and the filesystem roots it scans/organizes under. Reused across all
// five Modules fields rather than duplicated per content type, the same
// way config.OrganizeConfig is reused across content types in Pipeline.
type ModuleConfig struct {
	// Enabled gates whether this content type's module is active at all —
	// disabled means Roots is never scanned, regardless of what's
	// configured there.
	Enabled bool `mapstructure:"enabled"`

	// Roots are the directories this module's content lives under on disk.
	// Empty means nothing to scan even when Enabled.
	Roots []string `mapstructure:"roots"`
}

// Modules configures which content-type modules are active and which
// filesystem roots each scans/organizes under — see ops/purser.yaml's
// `modules:` block and .env.example's PURSER_MODULES_<MODULE>_ROOTS
// comment. Field keys (movies/tv/music/books/afterdark) are the
// operator-facing module names those files already use, and deliberately
// don't reuse domain.ContentType's own string values (movie/tv/music/
// book/adult) — named struct fields suit that mismatch better than
// Pipeline.Organize's map[domain.ContentType]OrganizeConfig, the same
// reasoning config.Sources already follows for its own provider fields.
type Modules struct {
	Movies    ModuleConfig `mapstructure:"movies"`
	TV        ModuleConfig `mapstructure:"tv"`
	Music     ModuleConfig `mapstructure:"music"`
	Books     ModuleConfig `mapstructure:"books"`
	AfterDark ModuleConfig `mapstructure:"afterdark"`
}

// DefaultModules returns Modules' defaults: every module disabled with no
// roots configured, the same opt-in-by-default posture every other
// external-facing component in this package starts from (Sources,
// Prowlarr, SABnzbd, QBittorrent). ops/purser.yaml's own `modules:` block
// (all five enabled with sample roots) is a non-default illustrative
// example, not what an unconfigured install starts from, per
// docs/adr/0010-configuration.md.
func DefaultModules() Modules {
	return Modules{}
}
