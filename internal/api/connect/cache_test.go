package apiconnect_test

import (
	"context"
	"errors"
	"purser/internal/ports"
	"purser/internal/service"
	"purser/pkg/cache"
	"testing"

	"connectrpc.com/connect"

	cachev1 "purser/gen/go/purser/cache/v1"

	apiconnect "purser/internal/api/connect"
)

type fakeCacheService struct {
	listStats []service.CacheStats
	listErr   error

	flushName    string
	flushFlushed []string
	flushErr     error
}

func (f *fakeCacheService) ListCacheStats(_ context.Context) ([]service.CacheStats, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.listStats, nil
}

func (f *fakeCacheService) FlushCache(_ context.Context, name string) ([]string, error) {
	f.flushName = name
	if f.flushErr != nil {
		return nil, f.flushErr
	}
	return f.flushFlushed, nil
}

func TestCacheHandler_ListCacheStats(t *testing.T) {
	svc := &fakeCacheService{listStats: []service.CacheStats{
		{Name: "musicbrainz", Stats: cache.Stats{Items: 1, Misses: 2}},
		{Name: "stashdb", Stats: cache.Stats{Items: 3, Hits: 5, Bytes: 1024, Sets: 6, Deletes: 1, Evictions: 2}},
	}}
	h := apiconnect.NewCacheHandler(svc, nil)

	resp, err := h.ListCacheStats(context.Background(), connect.NewRequest(&cachev1.ListCacheStatsRequest{}))
	if err != nil {
		t.Fatalf("ListCacheStats returned error: %v", err)
	}
	caches := resp.Msg.GetCaches()
	if len(caches) != 2 {
		t.Fatalf("ListCacheStats returned %d caches, want 2", len(caches))
	}

	byName := map[string]*cachev1.CacheStats{}
	for _, c := range caches {
		byName[c.GetName()] = c
	}

	if s := byName["stashdb"]; s.GetItems() != 3 || s.GetHits() != 5 || s.GetBytes() != 1024 || s.GetSets() != 6 || s.GetDeletes() != 1 || s.GetEvictions() != 2 {
		t.Fatalf("stashdb stats = %+v, want the fake's values", s)
	}
	if s := byName["musicbrainz"]; s.GetItems() != 1 || s.GetMisses() != 2 {
		t.Fatalf("musicbrainz stats = %+v, want the fake's values", s)
	}
}

func TestCacheHandler_ListCacheStats_Error(t *testing.T) {
	svc := &fakeCacheService{listErr: errors.New("boom")}
	h := apiconnect.NewCacheHandler(svc, nil)

	_, err := h.ListCacheStats(context.Background(), connect.NewRequest(&cachev1.ListCacheStatsRequest{}))
	var connErr *connect.Error
	if !errors.As(err, &connErr) {
		t.Fatalf("ListCacheStats returned %v, want a *connect.Error", err)
	}
	if connErr.Code() != connect.CodeInternal {
		t.Fatalf("ListCacheStats returned code %v, want %v", connErr.Code(), connect.CodeInternal)
	}
}

func TestCacheHandler_FlushCache(t *testing.T) {
	svc := &fakeCacheService{flushFlushed: []string{"stashdb"}}
	h := apiconnect.NewCacheHandler(svc, nil)

	resp, err := h.FlushCache(context.Background(), connect.NewRequest(&cachev1.FlushCacheRequest{Name: "stashdb"}))
	if err != nil {
		t.Fatalf("FlushCache returned error: %v", err)
	}
	if svc.flushName != "stashdb" {
		t.Fatalf("service received name %q, want %q", svc.flushName, "stashdb")
	}
	if got := resp.Msg.GetFlushed(); len(got) != 1 || got[0] != "stashdb" {
		t.Fatalf("FlushCache returned Flushed = %v, want [stashdb]", got)
	}
}

func TestCacheHandler_FlushCache_Empty(t *testing.T) {
	svc := &fakeCacheService{flushFlushed: []string{"musicbrainz", "stashdb"}}
	h := apiconnect.NewCacheHandler(svc, nil)

	resp, err := h.FlushCache(context.Background(), connect.NewRequest(&cachev1.FlushCacheRequest{}))
	if err != nil {
		t.Fatalf("FlushCache returned error: %v", err)
	}
	if svc.flushName != "" {
		t.Fatalf("service received name %q, want empty", svc.flushName)
	}
	if got := resp.Msg.GetFlushed(); len(got) != 2 {
		t.Fatalf("FlushCache returned Flushed = %v, want 2 entries", got)
	}
}

func TestCacheHandler_FlushCache_NotFound(t *testing.T) {
	svc := &fakeCacheService{flushErr: ports.ErrNotFound}
	h := apiconnect.NewCacheHandler(svc, nil)

	_, err := h.FlushCache(context.Background(), connect.NewRequest(&cachev1.FlushCacheRequest{Name: "nonexistent"}))
	var connErr *connect.Error
	if !errors.As(err, &connErr) {
		t.Fatalf("FlushCache returned %v, want a *connect.Error", err)
	}
	if connErr.Code() != connect.CodeNotFound {
		t.Fatalf("FlushCache returned code %v, want %v", connErr.Code(), connect.CodeNotFound)
	}
}
