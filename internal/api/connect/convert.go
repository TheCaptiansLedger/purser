package apiconnect

import (
	"math"
	"purser/internal/domain"

	v1 "purser/gen/go/purser/domain/v1"
)

// toInt32 clamps n into the int32 range before converting. Proto's int32
// fields here (Year, Width, Height, Priority, RuntimeSeconds) are always
// small in practice, but an explicit bounds check — rather than a bare
// conversion — is what actually rules out the overflow gosec's G115 flags,
// instead of just silencing the warning.
func toInt32(n int) int32 {
	switch {
	case n > math.MaxInt32:
		return math.MaxInt32
	case n < math.MinInt32:
		return math.MinInt32
	default:
		return int32(n)
	}
}

// deletionImpactRowsToProto converts a domain.DeletionImpact's rows —
// shared by every entity's GetXxxDeletionImpact handler, per
// docs/adr/0015-deletion-impact-and-composing-services.md.
func deletionImpactRowsToProto(rows []domain.DeletionImpactRow) []*v1.DeletionImpactRow {
	pb := make([]*v1.DeletionImpactRow, 0, len(rows))
	for _, r := range rows {
		pb = append(pb, &v1.DeletionImpactRow{Kind: r.Kind, Label: r.Label, Count: toInt32(r.Count), Blocking: r.Blocking})
	}
	return pb
}
