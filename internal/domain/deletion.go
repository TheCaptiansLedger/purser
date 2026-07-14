package domain

// DeletionImpactRow is one category of record that references a deletion
// target — e.g. "12 ItemPerson credits," "3 child Studios." Kind is
// machine-readable (matches the referrer's collection name, e.g.
// "item_person"); Label is human-readable, for a UI to show directly
// ("Credits"). Blocking is true when a non-zero Count prevents Delete from
// succeeding unless the caller explicitly requests cascade=true — set only
// for referrers with a required (non-nullable) foreign key to the target,
// where the referrer can't simply be unlinked because doing so would leave
// it invalid. See docs/adr/0015-deletion-impact-and-composing-services.md.
type DeletionImpactRow struct {
	Kind     string
	Label    string
	Count    int
	Blocking bool
}

// DeletionImpact is the full accounting of what references a deletion
// target, returned by a composing deletion service's GetDeletionImpact
// before Delete is called — the shape a UI shows as "deleting this will
// unlink it from 12 Credits and 3 Studios — continue?"
type DeletionImpact struct {
	Impacts []DeletionImpactRow
}
