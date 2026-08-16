package cacheregistry_test

import (
	"context"
	"purser/internal/adapters/cacheregistry"
	"purser/pkg/cache"
	"purser/pkg/cache/memory"
	"reflect"
	"testing"
)

func newTestCache(t *testing.T, name string) cache.Cache {
	t.Helper()
	c, err := memory.New(name, cache.DefaultConfig())
	if err != nil {
		t.Fatalf("memory.New(%q) returned error: %v", name, err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestRegistry_Names(t *testing.T) {
	stashdb := newTestCache(t, "stashdb")
	musicbrainz := newTestCache(t, "musicbrainz")

	r := cacheregistry.New(map[string]cache.Cache{
		"stashdb":     stashdb,
		"musicbrainz": musicbrainz,
	})

	got := r.Names()
	want := []string{"musicbrainz", "stashdb"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Names() = %v, want %v (sorted)", got, want)
	}
}

func TestRegistry_Names_Empty(t *testing.T) {
	r := cacheregistry.New(map[string]cache.Cache{})

	got := r.Names()
	if len(got) != 0 {
		t.Fatalf("Names() = %v, want empty", got)
	}
}

func TestRegistry_Names_ReturnsCopy(t *testing.T) {
	r := cacheregistry.New(map[string]cache.Cache{"stashdb": newTestCache(t, "stashdb")})

	names := r.Names()
	names[0] = "mutated"

	got := r.Names()
	if got[0] != "stashdb" {
		t.Fatalf("Names() returned a slice callers can mutate; got %v after external mutation", got)
	}
}

func TestRegistry_Cache_Found(t *testing.T) {
	stashdb := newTestCache(t, "stashdb")
	r := cacheregistry.New(map[string]cache.Cache{"stashdb": stashdb})

	got, ok := r.Cache("stashdb")
	if !ok {
		t.Fatal("Cache(\"stashdb\") returned ok=false, want true")
	}
	if got != stashdb {
		t.Fatal("Cache(\"stashdb\") did not return the registered instance")
	}

	// Sanity: the returned instance is the live, usable cache.Cache.
	ctx := context.Background()
	if err := got.Set(ctx, "k", []byte("v")); err != nil {
		t.Fatalf("Set on returned cache returned error: %v", err)
	}
	if v, ok, err := got.Get(ctx, "k"); err != nil || !ok || string(v) != "v" {
		t.Fatalf("Get on returned cache = (%q, %v, %v), want (\"v\", true, nil)", v, ok, err)
	}
}

func TestRegistry_Cache_NotFound(t *testing.T) {
	r := cacheregistry.New(map[string]cache.Cache{"stashdb": newTestCache(t, "stashdb")})

	_, ok := r.Cache("nonexistent")
	if ok {
		t.Fatal("Cache(\"nonexistent\") returned ok=true, want false")
	}
}
