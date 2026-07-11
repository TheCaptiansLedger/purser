// Package cachetest is the shared contract test suite for the cache.Cache
// port (see ADR 0003's contract-test convention). It is a normal buildable
// package, not a _test.go file, because Go test files cannot be imported
// across packages — every adapter (pkg/cache/memory today, others later)
// imports this from its own test file and runs it against its own
// constructor, proving Liskov substitutability without duplicating the
// assertions per adapter.
package cachetest

import (
	"context"
	"errors"
	"purser/pkg/cache"
	"testing"
	"time"
)

// NewCacheFunc returns a fresh, empty Cache configured with cfg for the
// duration of a single subtest.
type NewCacheFunc func(t *testing.T, cfg cache.Config) cache.Cache

// TestCache runs the shared Cache contract against newCache. Each check is
// its own top-level subtest so a single failure identifies exactly which
// part of the contract broke.
func TestCache(t *testing.T, newCache NewCacheFunc) {
	t.Helper()

	t.Run("get on empty cache misses", func(t *testing.T) { testGetOnEmptyMisses(t, newCache) })
	t.Run("set then get round-trips the value", func(t *testing.T) { testSetThenGet(t, newCache) })
	t.Run("set overwrites an existing key", func(t *testing.T) { testSetOverwrites(t, newCache) })
	t.Run("delete removes a key", func(t *testing.T) { testDeleteRemovesKey(t, newCache) })
	t.Run("delete on absent key is not an error", func(t *testing.T) { testDeleteAbsentKey(t, newCache) })
	t.Run("len reflects the number of entries", func(t *testing.T) { testLenReflectsEntries(t, newCache) })
	t.Run("entries beyond MaxItems evict the least recently used", func(t *testing.T) { testMaxItemsEviction(t, newCache) })
	t.Run("entries expire after DefaultTTL", func(t *testing.T) { testDefaultTTLExpiry(t, newCache) })
	t.Run("methods after close return ErrClosed", func(t *testing.T) { testMethodsAfterClose(t, newCache) })
}

func closeCache(t *testing.T, c cache.Cache) {
	t.Helper()
	if err := c.Close(); err != nil {
		t.Errorf("Close returned error: %v", err)
	}
}

func testGetOnEmptyMisses(t *testing.T, newCache NewCacheFunc) {
	c := newCache(t, cache.DefaultConfig())
	defer closeCache(t, c)

	_, ok, err := c.Get(context.Background(), "missing")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if ok {
		t.Fatal("Get on empty cache reported a hit")
	}
}

func testSetThenGet(t *testing.T, newCache NewCacheFunc) {
	c := newCache(t, cache.DefaultConfig())
	defer closeCache(t, c)

	ctx := context.Background()
	if err := c.Set(ctx, "key", []byte("value")); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	got, ok, err := c.Get(ctx, "key")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if !ok {
		t.Fatal("Get reported a miss after Set")
	}
	if string(got) != "value" {
		t.Fatalf("Get returned %q, want %q", got, "value")
	}
}

func testSetOverwrites(t *testing.T, newCache NewCacheFunc) {
	c := newCache(t, cache.DefaultConfig())
	defer closeCache(t, c)

	ctx := context.Background()
	if err := c.Set(ctx, "key", []byte("first")); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	if err := c.Set(ctx, "key", []byte("second")); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	got, ok, err := c.Get(ctx, "key")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if !ok {
		t.Fatal("Get reported a miss after overwrite")
	}
	if string(got) != "second" {
		t.Fatalf("Get returned %q, want %q", got, "second")
	}

	n, err := c.Len(ctx)
	if err != nil {
		t.Fatalf("Len returned error: %v", err)
	}
	if n != 1 {
		t.Fatalf("Len = %d after overwrite, want 1", n)
	}
}

func testDeleteRemovesKey(t *testing.T, newCache NewCacheFunc) {
	c := newCache(t, cache.DefaultConfig())
	defer closeCache(t, c)

	ctx := context.Background()
	if err := c.Set(ctx, "key", []byte("value")); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	if err := c.Delete(ctx, "key"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	_, ok, err := c.Get(ctx, "key")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if ok {
		t.Fatal("Get reported a hit after Delete")
	}
}

func testDeleteAbsentKey(t *testing.T, newCache NewCacheFunc) {
	c := newCache(t, cache.DefaultConfig())
	defer closeCache(t, c)

	if err := c.Delete(context.Background(), "missing"); err != nil {
		t.Fatalf("Delete on absent key returned error: %v", err)
	}
}

func testLenReflectsEntries(t *testing.T, newCache NewCacheFunc) {
	c := newCache(t, cache.DefaultConfig())
	defer closeCache(t, c)

	ctx := context.Background()
	for _, k := range []string{"a", "b", "c"} {
		if err := c.Set(ctx, k, []byte(k)); err != nil {
			t.Fatalf("Set(%q) returned error: %v", k, err)
		}
	}

	n, err := c.Len(ctx)
	if err != nil {
		t.Fatalf("Len returned error: %v", err)
	}
	if n != 3 {
		t.Fatalf("Len = %d, want 3", n)
	}
}

func testMaxItemsEviction(t *testing.T, newCache NewCacheFunc) {
	cfg := cache.DefaultConfig()
	cfg.MaxItems = 2
	c := newCache(t, cfg)
	defer closeCache(t, c)

	ctx := context.Background()
	mustSet(t, c, ctx, "a", "a")
	mustSet(t, c, ctx, "b", "b")
	mustSet(t, c, ctx, "c", "c") // should evict "a"

	_, ok, err := c.Get(ctx, "a")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if ok {
		t.Fatal("oldest entry was not evicted after exceeding MaxItems")
	}

	for _, k := range []string{"b", "c"} {
		_, ok, err := c.Get(ctx, k)
		if err != nil {
			t.Fatalf("Get(%q) returned error: %v", k, err)
		}
		if !ok {
			t.Fatalf("Get(%q) missed, want hit", k)
		}
	}
}

func testDefaultTTLExpiry(t *testing.T, newCache NewCacheFunc) {
	cfg := cache.DefaultConfig()
	cfg.DefaultTTL = 10 * time.Millisecond
	c := newCache(t, cfg)
	defer closeCache(t, c)

	ctx := context.Background()
	if err := c.Set(ctx, "key", []byte("value")); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	_, ok, err := c.Get(ctx, "key")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if ok {
		t.Fatal("Get reported a hit after DefaultTTL elapsed")
	}
}

func testMethodsAfterClose(t *testing.T, newCache NewCacheFunc) {
	c := newCache(t, cache.DefaultConfig())
	if err := c.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	ctx := context.Background()
	if _, _, err := c.Get(ctx, "key"); !errors.Is(err, cache.ErrClosed) {
		t.Fatalf("Get after Close returned %v, want ErrClosed", err)
	}
	if err := c.Set(ctx, "key", []byte("v")); !errors.Is(err, cache.ErrClosed) {
		t.Fatalf("Set after Close returned %v, want ErrClosed", err)
	}
	if err := c.Delete(ctx, "key"); !errors.Is(err, cache.ErrClosed) {
		t.Fatalf("Delete after Close returned %v, want ErrClosed", err)
	}
	if _, err := c.Len(ctx); !errors.Is(err, cache.ErrClosed) {
		t.Fatalf("Len after Close returned %v, want ErrClosed", err)
	}
}

func mustSet(t *testing.T, c cache.Cache, ctx context.Context, key, value string) {
	t.Helper()
	if err := c.Set(ctx, key, []byte(value)); err != nil {
		t.Fatalf("Set(%q) returned error: %v", key, err)
	}
}
