package apiconnect

import (
	"purser/internal/service"

	cachev1 "purser/gen/go/purser/cache/v1"
)

// cacheStatsToProto converts a service.CacheStats to its wire shape.
func cacheStatsToProto(cs service.CacheStats) *cachev1.CacheStats {
	return &cachev1.CacheStats{
		Name:      cs.Name,
		Items:     toInt32(cs.Items),
		Bytes:     cs.Bytes,
		Hits:      cs.Hits,
		Misses:    cs.Misses,
		Sets:      cs.Sets,
		Deletes:   cs.Deletes,
		Evictions: cs.Evictions,
	}
}

// cacheStatsSliceToProto converts a slice of service.CacheStats to its wire
// shape, preserving order.
func cacheStatsSliceToProto(stats []service.CacheStats) []*cachev1.CacheStats {
	out := make([]*cachev1.CacheStats, len(stats))
	for i, cs := range stats {
		out[i] = cacheStatsToProto(cs)
	}
	return out
}
