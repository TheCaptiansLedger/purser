package config

// AfterDark configures AfterDark's scan-pipeline decide/persist behavior —
// distinct from Sources.StashDB/Sources.ThePornDB, which configure the
// providers' own adapters (auth, enablement). See
// docs/adr/0024-pipeline-core.md, docs/adr/0027-provider-independence.md,
// AD7 (issue #561).
type AfterDark struct {
	// ProviderPriority is the order the AfterDark Persister resolves
	// scalar-field conflicts (Title/Overview/Date, Studio) when more than
	// one provider's candidate independently cleared the confidence
	// threshold for the same file — the first entry present among the
	// cleared candidates' own sources wins. Values are the same
	// lowercase provider names Identify already tags every candidate with
	// (Metadata["source"]: "stashdb", "tpdb"). This is a narrow, deliberate
	// exception to docs/adr/0027-provider-independence.md's "no server-side
	// picking a winner" rule — that ADR's own text carves out exactly this
	// case ("Not in scope: ... an unattended scan legitimately uses
	// provider data server-side") — never a hardcoded pick: an operator's
	// own configured order, applied consistently.
	ProviderPriority []string `mapstructure:"provider_priority"`
}

// DefaultAfterDark returns AfterDark's defaults: StashDB before ThePornDB,
// an arbitrary-but-documented starting order (not asserted correct), like
// every other unvalidated starting constant this pipeline carries.
func DefaultAfterDark() AfterDark {
	return AfterDark{ProviderPriority: []string{"stashdb", "tpdb"}}
}
