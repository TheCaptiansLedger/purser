package domain

import (
	"fmt"
	"strings"
)

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

// Summarize builds the human-readable summary sentence for the impact report.
// Adapters populate Impacts with raw counts and labels; the service calls this
// once it has the entity name so presentation logic stays out of the storage layer.
func (d *DeletionImpact) Summarize(name string) string {
	if len(d.Impacts) == 0 {
		return "Deleting " + name + " will permanently remove it from the library."
	}
	parts := make([]string, 0, len(d.Impacts))
	for _, row := range d.Impacts {
		parts = append(parts, fmt.Sprintf("%d %s", row.Count, row.Label))
	}
	return fmt.Sprintf("Deleting %s will permanently remove %s.", name, strings.Join(parts, " and "))
}
