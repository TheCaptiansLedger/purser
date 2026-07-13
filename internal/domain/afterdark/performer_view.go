package afterdark

import "purser/internal/domain"

// PerformerView composes a kernel domain.Person with its AfterDark
// PerformerProfile, per docs/technical/shared-domain-model.md Part 3's
// Profile pattern. It is a read-only, assembled-on-demand shape — never
// persisted as its own row, never validated, no repository of its own.
// internal/service.AfterDarkBrowseService is the only thing that
// constructs one; declaring the type here (rather than in the service
// package) is what lets internal/api/connect depend on it as shared
// vocabulary, the same way it already depends on domain.Item/domain.Person
// and afterdark.PerformerProfile, without depending on internal/service
// itself.
type PerformerView struct {
	Person  *domain.Person
	Profile *PerformerProfile
}
