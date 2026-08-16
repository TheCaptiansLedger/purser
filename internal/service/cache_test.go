package service_test

import (
	"context"
	"errors"
	"purser/internal/ports"
	"purser/internal/service"
	"purser/pkg/cache"
	"reflect"
	"sort"
	"testing"
)

// fakeCacheRegistry is a fake ports.CacheRegistry, per docs/adr/0003's
// "services: unit tests against fake port implementations, not real
// adapters" rule.
type fakeCacheRegistry struct {
	caches map[string]cache.Cache
}

func (f *fakeCacheRegistry) Names() []string {
	names := make([]string, 0, len(f.caches))
	for name := range f.caches {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (f *fakeCacheRegistry) Cache(name string) (cache.Cache, bool) {
	c, ok := f.caches[name]
	return c, ok
}

// fakeCache is a fake cache.Cache exercising only what CacheService calls:
// Stats and Flush.
type fakeCache struct {
	stats    cache.Stats
	statsErr error

	flushed  bool
	flushErr error
}

func (f *fakeCache) Get(_ context.Context, _ string) ([]byte, bool, error) { return nil, false, nil }
func (f *fakeCache) Set(_ context.Context, _ string, _ []byte) error       { return nil }
func (f *fakeCache) Delete(_ context.Context, _ string) error              { return nil }
func (f *fakeCache) Len(_ context.Context) (int, error)                    { return 0, nil }
func (f *fakeCache) Close() error                                          { return nil }

func (f *fakeCache) Stats(_ context.Context) (cache.Stats, error) {
	if f.statsErr != nil {
		return cache.Stats{}, f.statsErr
	}
	return f.stats, nil
}

func (f *fakeCache) Flush(_ context.Context) error {
	f.flushed = true
	return f.flushErr
}

var (
	_ cache.Cache         = (*fakeCache)(nil)
	_ ports.CacheRegistry = (*fakeCacheRegistry)(nil)
)

func TestCacheService_ListCacheStats(t *testing.T) {
	stashdb := &fakeCache{stats: cache.Stats{Items: 3, Hits: 5}}
	musicbrainz := &fakeCache{stats: cache.Stats{Items: 1, Misses: 2}}
	registry := &fakeCacheRegistry{caches: map[string]cache.Cache{
		"stashdb":     stashdb,
		"musicbrainz": musicbrainz,
	}}
	svc := service.NewCacheService(registry)

	got, err := svc.ListCacheStats(context.Background())
	if err != nil {
		t.Fatalf("ListCacheStats returned error: %v", err)
	}

	want := []service.CacheStats{
		{Name: "musicbrainz", Stats: cache.Stats{Items: 1, Misses: 2}},
		{Name: "stashdb", Stats: cache.Stats{Items: 3, Hits: 5}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ListCacheStats = %+v, want %+v (sorted by name)", got, want)
	}
}

func TestCacheService_ListCacheStats_Empty(t *testing.T) {
	svc := service.NewCacheService(&fakeCacheRegistry{caches: map[string]cache.Cache{}})

	got, err := svc.ListCacheStats(context.Background())
	if err != nil {
		t.Fatalf("ListCacheStats returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ListCacheStats = %+v, want empty", got)
	}
}

func TestCacheService_ListCacheStats_StatsError(t *testing.T) {
	wantErr := errors.New("boom")
	registry := &fakeCacheRegistry{caches: map[string]cache.Cache{
		"stashdb": &fakeCache{statsErr: wantErr},
	}}
	svc := service.NewCacheService(registry)

	_, err := svc.ListCacheStats(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("ListCacheStats error = %v, want wrapping %v", err, wantErr)
	}
}

func TestCacheService_FlushCache_SingleName(t *testing.T) {
	stashdb := &fakeCache{}
	musicbrainz := &fakeCache{}
	registry := &fakeCacheRegistry{caches: map[string]cache.Cache{
		"stashdb":     stashdb,
		"musicbrainz": musicbrainz,
	}}
	svc := service.NewCacheService(registry)

	got, err := svc.FlushCache(context.Background(), "stashdb")
	if err != nil {
		t.Fatalf("FlushCache returned error: %v", err)
	}
	if want := []string{"stashdb"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("FlushCache flushed = %v, want %v", got, want)
	}
	if !stashdb.flushed {
		t.Error("stashdb was not flushed")
	}
	if musicbrainz.flushed {
		t.Error("musicbrainz was flushed, want untouched")
	}
}

func TestCacheService_FlushCache_EmptyName_FlushesAll(t *testing.T) {
	stashdb := &fakeCache{}
	musicbrainz := &fakeCache{}
	registry := &fakeCacheRegistry{caches: map[string]cache.Cache{
		"stashdb":     stashdb,
		"musicbrainz": musicbrainz,
	}}
	svc := service.NewCacheService(registry)

	got, err := svc.FlushCache(context.Background(), "")
	if err != nil {
		t.Fatalf("FlushCache returned error: %v", err)
	}
	if want := []string{"musicbrainz", "stashdb"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("FlushCache flushed = %v, want %v (sorted)", got, want)
	}
	if !stashdb.flushed || !musicbrainz.flushed {
		t.Error("FlushCache(\"\") did not flush every registered cache")
	}
}

func TestCacheService_FlushCache_UnknownName(t *testing.T) {
	svc := service.NewCacheService(&fakeCacheRegistry{caches: map[string]cache.Cache{
		"stashdb": &fakeCache{},
	}})

	_, err := svc.FlushCache(context.Background(), "nonexistent")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("FlushCache error = %v, want ports.ErrNotFound", err)
	}
}

func TestCacheService_FlushCache_FlushError(t *testing.T) {
	wantErr := errors.New("boom")
	registry := &fakeCacheRegistry{caches: map[string]cache.Cache{
		"stashdb": &fakeCache{flushErr: wantErr},
	}}
	svc := service.NewCacheService(registry)

	_, err := svc.FlushCache(context.Background(), "stashdb")
	if !errors.Is(err, wantErr) {
		t.Fatalf("FlushCache error = %v, want wrapping %v", err, wantErr)
	}
}
