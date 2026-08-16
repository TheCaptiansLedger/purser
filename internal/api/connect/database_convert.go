package apiconnect

import (
	"purser/internal/ports"

	databasev1 "purser/gen/go/purser/database/v1"
)

// databaseInfoToProto converts a ports.DatabaseInfo to its wire shape.
func databaseInfoToProto(info ports.DatabaseInfo) *databasev1.GetDatabaseInfoResponse {
	counts := make(map[string]int64, len(info.CollectionCounts))
	for k, v := range info.CollectionCounts {
		counts[k] = v
	}
	return &databasev1.GetDatabaseInfoResponse{
		Driver:           info.Driver,
		Version:          info.Version,
		StorageSizeBytes: info.StorageSizeBytes,
		CollectionCounts: counts,
	}
}
