package domain

// DeletionMode describes the effect of deleting an entity.
// Destroy means the entity and its descendants are permanently removed.
// Unlink means the entity itself is deleted but content it references is untouched.
type DeletionMode string

// Deletion mode constants describing the effect of deleting an entity.
const (
	DeletionModeDestroy DeletionMode = "destroy"
	DeletionModeUnlink  DeletionMode = "unlink"
)

// DeletionImpactRow is one line in a deletion impact report.
type DeletionImpactRow struct {
	Kind  string `json:"kind"`
	Count int    `json:"count"`
	Label string `json:"label"`
}

// DeletionImpact summarises what will happen when an entity is deleted.
type DeletionImpact struct {
	Mode    DeletionMode        `json:"mode"`
	Summary string              `json:"summary"`
	Impacts []DeletionImpactRow `json:"impacts"`
}
